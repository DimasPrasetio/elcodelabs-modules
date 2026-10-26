// Package application holds the rbac module's use cases: roles, their permissions, their
// members, and answering "may this user do that?".
package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/rbac/domain"
	"__GO_MODULE__/internal/modules/rbac/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/validator"
)

// ErrNotFound and ErrNameTaken are what a Repository reports.
var (
	ErrNotFound  = errors.New("role not found")
	ErrNameTaken = errors.New("role name already in use")
)

// Repository is the roles, role_permissions, and user_roles tables.
type Repository interface {
	ListRoles(ctx context.Context) ([]domain.Role, error)
	GetRole(ctx context.Context, id int64) (domain.Role, error)
	RoleIDByName(ctx context.Context, name string) (int64, error)
	CreateRole(ctx context.Context, name, description string, permissions []string) (int64, error)
	UpdateRole(ctx context.Context, id int64, name, description string, permissions []string) error
	DeleteRole(ctx context.Context, id int64) error
	Members(ctx context.Context, roleID int64) ([]int64, error)
	AddMember(ctx context.Context, roleID, userID int64) error
	RemoveMember(ctx context.Context, roleID, userID int64) error
	PermissionsOfUser(ctx context.Context, userID int64) ([]string, error)
}

// Member is a user holding a role. A user the directory no longer knows is kept, marked Missing,
// so an admin can still remove the assignment.
type Member struct {
	ports.User
	Missing bool
}

// Service is the rbac module's use cases.
type Service struct {
	repo    Repository
	users   ports.UserDirectory
	catalog func() []kernel.Permission
	audit   Auditor
	ttl     time.Duration
	now     func() time.Time

	mu    sync.Mutex
	cache map[int64]cached
}

type cached struct {
	granted map[string]bool
	until   time.Time
}

// Auditor records changes to roles and members — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// NewService returns the service. catalog lists every permission modules declared.
func NewService(repo Repository, users ports.UserDirectory, catalog func() []kernel.Permission, audit Auditor) *Service {
	return &Service{repo: repo, users: users, catalog: catalog, audit: audit, ttl: 30 * time.Second, now: time.Now, cache: map[int64]cached{}}
}

func roleTarget(id int64) string { return fmt.Sprintf("role:%d", id) }

// Catalog is every permission that can be granted.
func (s *Service) Catalog() []kernel.Permission { return s.catalog() }

func (s *Service) ListRoles(ctx context.Context) ([]domain.Role, error) { return s.repo.ListRoles(ctx) }

func (s *Service) GetRole(ctx context.Context, id int64) (domain.Role, error) {
	r, err := s.repo.GetRole(ctx, id)
	return r, notFound(err)
}

// RoleInput is what creating or changing a role takes.
type RoleInput struct {
	Name        string
	Description string
	Permissions []string
}

func (s *Service) clean(in RoleInput) (RoleInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	var v validator.Validator
	domain.ValidateRole(&v, in.Name, in.Description)
	known := map[string]bool{}
	for _, p := range s.catalog() {
		known[p.Name] = true
	}
	seen := map[string]bool{}
	var perms []string
	for _, p := range in.Permissions {
		if seen[p] {
			continue
		}
		seen[p] = true
		v.Check(known[p], "permissions", "izin tidak dikenal: "+p)
		perms = append(perms, p)
	}
	sort.Strings(perms)
	in.Permissions = perms
	return in, v.Err()
}

// errBeyondOwn refuses handing out (or taking away) more than the actor holds — the rule that
// keeps rbac.manage from being a path to every permission.
var errBeyondOwn = apperror.Forbidden("Anda hanya dapat mengelola izin dan peran yang Anda pegang sendiri")

// holds reports whether actor holds every one of perms.
func (s *Service) holds(ctx context.Context, actor int64, perms []string) bool {
	granted, err := s.Granted(ctx, actor)
	if err != nil {
		return false
	}
	for _, p := range perms {
		if !domain.Grants(granted, p) {
			return false
		}
	}
	return true
}

// holdsRole reports whether actor holds everything the role grants (the Administrator role:
// every permission, so only an administrator).
func (s *Service) holdsRole(ctx context.Context, actor int64, r domain.Role) bool {
	if r.IsSystem {
		return s.holds(ctx, actor, []string{domain.Wildcard})
	}
	return s.holds(ctx, actor, r.Permissions)
}

// Covers reports whether actor holds every permission target holds — whether actor may act on
// target's account (kernel Authorizer.Covers). A lookup failure denies.
func (s *Service) Covers(ctx context.Context, actor, target int64) bool {
	if actor == target {
		return true
	}
	theirs, err := s.Granted(ctx, target)
	if err != nil {
		return false
	}
	perms := make([]string, 0, len(theirs))
	for p := range theirs {
		perms = append(perms, p)
	}
	return s.holds(ctx, actor, perms)
}

func (s *Service) CreateRole(ctx context.Context, actor int64, in RoleInput) (domain.Role, error) {
	in, err := s.clean(in)
	if err != nil {
		return domain.Role{}, err
	}
	if !s.holds(ctx, actor, in.Permissions) {
		return domain.Role{}, errBeyondOwn
	}
	id, err := s.repo.CreateRole(ctx, in.Name, in.Description, in.Permissions)
	if err != nil {
		return domain.Role{}, nameTaken(err)
	}
	s.invalidate()
	s.audit.Record(ctx, kernel.AuditEntry{Module: "rbac", Action: "role.created", Target: roleTarget(id), Summary: "Membuat peran " + in.Name,
		Data: map[string]any{"permissions": in.Permissions}})
	return s.repo.GetRole(ctx, id)
}

func (s *Service) UpdateRole(ctx context.Context, actor, id int64, in RoleInput) (domain.Role, error) {
	current, err := s.repo.GetRole(ctx, id)
	if err != nil {
		return domain.Role{}, notFound(err)
	}
	if current.IsSystem {
		return domain.Role{}, apperror.Conflict("Peran Administrator selalu memegang semua izin dan tidak dapat diubah")
	}
	if in, err = s.clean(in); err != nil {
		return domain.Role{}, err
	}
	if !s.holdsRole(ctx, actor, current) || !s.holds(ctx, actor, in.Permissions) {
		return domain.Role{}, errBeyondOwn
	}
	if err := s.repo.UpdateRole(ctx, id, in.Name, in.Description, in.Permissions); err != nil {
		return domain.Role{}, nameTaken(notFound(err))
	}
	s.invalidate()
	s.audit.Record(ctx, kernel.AuditEntry{Module: "rbac", Action: "role.updated", Target: roleTarget(id), Summary: "Mengubah peran " + in.Name,
		Data: map[string]any{"name": map[string]any{"before": current.Name, "after": in.Name}, "permissions": map[string]any{"before": current.Permissions, "after": in.Permissions}}})
	return s.repo.GetRole(ctx, id)
}

func (s *Service) DeleteRole(ctx context.Context, actor, id int64) error {
	r, err := s.repo.GetRole(ctx, id)
	if err != nil {
		return notFound(err)
	}
	if r.IsSystem {
		return apperror.Conflict("Peran Administrator tidak dapat dihapus")
	}
	if !s.holdsRole(ctx, actor, r) {
		return errBeyondOwn
	}
	if err := s.repo.DeleteRole(ctx, id); err != nil {
		return notFound(err)
	}
	s.invalidate()
	s.audit.Record(ctx, kernel.AuditEntry{Module: "rbac", Action: "role.deleted", Target: roleTarget(id), Summary: "Menghapus peran " + r.Name,
		Data: map[string]any{"permissions": r.Permissions}})
	return nil
}

// Members returns the users holding the role.
func (s *Service) Members(ctx context.Context, roleID int64) ([]Member, error) {
	if _, err := s.GetRole(ctx, roleID); err != nil {
		return nil, err
	}
	ids, err := s.repo.Members(ctx, roleID)
	if err != nil {
		return nil, err
	}
	out := make([]Member, 0, len(ids))
	for _, id := range ids {
		u, err := s.users.Get(ctx, id)
		switch {
		case errors.Is(err, ports.ErrUserNotFound):
			out = append(out, Member{User: ports.User{ID: id}, Missing: true})
		case err != nil:
			return nil, err
		default:
			out = append(out, Member{User: u})
		}
	}
	return out, nil
}

// AddMember gives the role to a user the directory knows — only a role whose permissions the
// actor holds, so nobody can make themselves (or a friend) Administrator.
func (s *Service) AddMember(ctx context.Context, actor, roleID, userID int64) error {
	r, err := s.GetRole(ctx, roleID)
	if err != nil {
		return err
	}
	if !s.holdsRole(ctx, actor, r) {
		return errBeyondOwn
	}
	if _, err := s.users.Get(ctx, userID); errors.Is(err, ports.ErrUserNotFound) {
		return apperror.NotFound("Pengguna tidak ditemukan")
	} else if err != nil {
		return err
	}
	if err := s.repo.AddMember(ctx, roleID, userID); err != nil {
		return err
	}
	s.invalidate()
	s.audit.Record(ctx, kernel.AuditEntry{Module: "rbac", Action: "role.member_added", Target: fmt.Sprintf("user:%d", userID), Summary: "Memberi peran " + r.Name,
		Data: map[string]any{"role_id": roleID, "role": r.Name}})
	return nil
}

// RemoveMember takes the role away. The last Administrator cannot lose it — someone must
// always be able to manage roles.
func (s *Service) RemoveMember(ctx context.Context, actor, roleID, userID int64) error {
	r, err := s.GetRole(ctx, roleID)
	if err != nil {
		return err
	}
	if !s.holdsRole(ctx, actor, r) {
		return errBeyondOwn
	}
	if r.IsSystem {
		ids, err := s.repo.Members(ctx, roleID)
		if err != nil {
			return err
		}
		if len(ids) <= 1 {
			return apperror.Conflict("Minimal satu pengguna harus tetap memegang peran Administrator")
		}
	}
	if err := s.repo.RemoveMember(ctx, roleID, userID); err != nil {
		return notFound(err)
	}
	s.invalidate()
	s.audit.Record(ctx, kernel.AuditEntry{Module: "rbac", Action: "role.member_removed", Target: fmt.Sprintf("user:%d", userID), Summary: "Mencabut peran " + r.Name,
		Data: map[string]any{"role_id": roleID, "role": r.Name}})
	return nil
}

// SearchUsers finds users to give a role to.
func (s *Service) SearchUsers(ctx context.Context, q string) ([]ports.User, error) {
	return s.users.Search(ctx, strings.TrimSpace(q), 10)
}

// Granted returns the user's permissions, cached for a short time; every change to roles or
// members clears the cache.
func (s *Service) Granted(ctx context.Context, userID int64) (map[string]bool, error) {
	s.mu.Lock()
	c, ok := s.cache[userID]
	s.mu.Unlock()
	if ok && s.now().Before(c.until) {
		return c.granted, nil
	}
	perms, err := s.repo.PermissionsOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	granted := make(map[string]bool, len(perms))
	for _, p := range perms {
		granted[p] = true
	}
	s.mu.Lock()
	s.cache[userID] = cached{granted: granted, until: s.now().Add(s.ttl)}
	s.mu.Unlock()
	return granted, nil
}

// Effective lists what the user may do: every catalog permission they are granted.
func (s *Service) Effective(ctx context.Context, userID int64) ([]string, error) {
	granted, err := s.Granted(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, p := range s.catalog() {
		if domain.Grants(granted, p.Name) {
			out = append(out, p.Name)
		}
	}
	return out, nil
}

// Can reports whether the user holds permission. A lookup failure denies.
func (s *Service) Can(ctx context.Context, userID int64, permission string) bool {
	granted, err := s.Granted(ctx, userID)
	return err == nil && domain.Grants(granted, permission)
}

// Bootstrap gives the Administrator role to the user with email when no active user holds it
// — how a fresh install gets someone who can manage roles, and how an install whose only
// administrators were deactivated or removed gets one back.
func (s *Service) Bootstrap(ctx context.Context, email string) (assigned bool, err error) {
	adminID, err := s.repo.RoleIDByName(ctx, domain.AdminRole)
	if err != nil {
		return false, err
	}
	ids, err := s.repo.Members(ctx, adminID)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		u, err := s.users.Get(ctx, id)
		if err == nil && u.IsActive {
			return false, nil
		}
		if err != nil && !errors.Is(err, ports.ErrUserNotFound) {
			return false, err
		}
	}
	u, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return false, err
	}
	if !u.IsActive {
		return false, errors.New("the bootstrap administrator's account is deactivated")
	}
	if err := s.repo.AddMember(ctx, adminID, u.ID); err != nil {
		return false, err
	}
	s.invalidate()
	return true, nil
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cache = map[int64]cached{}
	s.mu.Unlock()
}

func notFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("Peran tidak ditemukan")
	}
	return err
}

func nameTaken(err error) error {
	if errors.Is(err, ErrNameTaken) {
		return apperror.Validation([]validator.FieldError{{Field: "name", Message: "sudah dipakai"}})
	}
	return err
}
