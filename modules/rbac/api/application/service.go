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

// Repository is the roles, role_permissions, and user_roles tables. Roles are defined once for
// the app; who holds them is per company — across all of it (branch 0) or in one branch.
type Repository interface {
	// LockRole locks the role's row until the transaction in ctx ends — exclusively to change
	// the role, shared to give it to someone — or reports ErrNotFound.
	LockRole(ctx context.Context, id int64, exclusive bool) error
	// RoleCompanies lists the companies where the role has members.
	RoleCompanies(ctx context.Context, roleID int64) ([]int64, error)
	// LockMembers is Members, locking the rows until the transaction in ctx ends.
	LockMembers(ctx context.Context, company, roleID int64) ([]Assignment, error)
	ListRoles(ctx context.Context, company int64) ([]domain.Role, error)
	GetRole(ctx context.Context, company, id int64) (domain.Role, error)
	RoleIDByName(ctx context.Context, name string) (int64, error)
	CreateRole(ctx context.Context, name, description string, permissions []string) (int64, error)
	UpdateRole(ctx context.Context, id int64, name, description string, permissions []string) error
	DeleteRole(ctx context.Context, id int64) error
	Members(ctx context.Context, company, roleID int64) ([]Assignment, error)
	AddMember(ctx context.Context, company, branch, roleID, userID int64) error
	RemoveMember(ctx context.Context, company, branch, roleID, userID int64) error
	// GrantsOfUser is every permission the user holds, in every company.
	GrantsOfUser(ctx context.Context, userID int64) ([]Grant, error)
}

// Transactor runs fn in one transaction — the kernel's d.Tx.
type Transactor interface {
	Do(ctx context.Context, fn func(ctx context.Context) error, opts ...kernel.TxOption) error
	AfterCommit(ctx context.Context, fn func(ctx context.Context))
	InTx(ctx context.Context) bool
}

// Scopes says whether a user is placed in a company or branch — the kernel's d.Scope, answered
// by the organization module (without one, only company 1 exists).
type Scopes interface {
	Resolve(ctx context.Context, userID int64, requested string) (kernel.Scope, error)
}

// Assignment is a user holding a role in the company: everywhere in it (BranchID 0) or in one
// branch.
type Assignment struct {
	UserID   int64
	BranchID int64
}

// Grant is one permission a user holds, and where.
type Grant struct {
	CompanyID  int64
	BranchID   int64 // 0: every branch of the company
	Permission string
}

// Member is a user holding a role. A user the directory no longer knows is kept, marked Missing,
// so an admin can still remove the assignment.
type Member struct {
	ports.User
	BranchID int64
	Missing  bool
}

// Service is the rbac module's use cases.
type Service struct {
	repo    Repository
	tx      Transactor
	scopes  Scopes
	users   ports.UserDirectory
	catalog func() []kernel.Permission
	audit   Auditor
	ttl     time.Duration
	now     func() time.Time

	mu    sync.Mutex
	cache map[grantKey]cached
	gen   uint64 // bumped by every change: a lookup begun before one is not cached
}

type grantKey struct{ user int64 }

type cached struct {
	grants []Grant
	until  time.Time
}

// Auditor records changes to roles and members — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// NewService returns the service. catalog lists every permission modules declared.
func NewService(repo Repository, tx Transactor, scopes Scopes, users ports.UserDirectory, catalog func() []kernel.Permission, audit Auditor) *Service {
	return &Service{repo: repo, tx: tx, scopes: scopes, users: users, catalog: catalog, audit: audit, ttl: 30 * time.Second, now: time.Now, cache: map[grantKey]cached{}}
}

func roleTarget(id int64) string { return fmt.Sprintf("role:%d", id) }

// Catalog is every permission that can be granted.
func (s *Service) Catalog() []kernel.Permission { return s.catalog() }

// company is the company the request works in (d.Auth resolved its scope).
func company(ctx context.Context) int64 { return kernel.ScopeFrom(ctx).CompanyID }

func (s *Service) ListRoles(ctx context.Context) ([]domain.Role, error) {
	return s.repo.ListRoles(ctx, company(ctx))
}

func (s *Service) GetRole(ctx context.Context, id int64) (domain.Role, error) {
	r, err := s.repo.GetRole(ctx, company(ctx), id)
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

// at is the permissions grants give in branch of company: a grant across the company applies
// everywhere in it, a branch grant only in its branch. Branch 0 (the whole company) counts
// company-wide grants only — holding a role in one branch never makes anyone a company-wide
// manager.
func at(grants []Grant, company, branch int64) map[string]bool {
	out := map[string]bool{}
	for _, g := range grants {
		if g.CompanyID == company && (g.BranchID == 0 || (branch != 0 && g.BranchID == branch)) {
			out[g.Permission] = true
		}
	}
	return out
}

// holds reports whether actor holds every one of perms in branch of company.
func (s *Service) holds(ctx context.Context, actor, company, branch int64, perms ...string) bool {
	grants, err := s.grants(ctx, actor)
	if err != nil {
		return false
	}
	granted := at(grants, company, branch)
	for _, p := range perms {
		if !domain.Grants(granted, p) {
			return false
		}
	}
	return true
}

// rolePerms is what holding role r takes: everything it grants (the Administrator role: every
// permission).
func rolePerms(r domain.Role) []string {
	if r.IsSystem {
		return []string{domain.Wildcard}
	}
	return r.Permissions
}

// mayEditRole reports whether actor may change or delete a role holding perms. Roles are
// defined once for the app, so a change reaches every company where the role has members:
// actor needs rbac.manage and perms company-wide in each of them, and in the active company.
func (s *Service) mayEditRole(ctx context.Context, actor, roleID int64, perms []string) (bool, error) {
	companies := []int64{company(ctx)}
	if roleID != 0 {
		others, err := s.repo.RoleCompanies(ctx, roleID)
		if err != nil {
			return false, err
		}
		companies = append(companies, others...)
	}
	for _, c := range companies {
		if !s.holds(ctx, actor, c, 0, append([]string{domain.PermManage}, perms...)...) {
			return false, nil
		}
	}
	return true, nil
}

// Covers reports whether actor may act on target's account (kernel Authorizer.Covers: reset a
// password, deactivate). An account is one across every company and branch, so acting on it is a
// company-wide act: actor needs, company-wide, every permission target holds, wherever target
// holds it — and, for an account holding nothing, some company-wide role in the active company
// (holding a role in one branch never reaches accounts). A lookup failure denies.
func (s *Service) Covers(ctx context.Context, actor, target int64) bool {
	if actor == target {
		return true
	}
	theirs, err := s.grants(ctx, target)
	if err != nil {
		return false
	}
	mine, err := s.grants(ctx, actor)
	if err != nil {
		return false
	}
	if len(theirs) == 0 {
		return len(at(mine, company(ctx), 0)) > 0
	}
	for _, g := range theirs {
		if !domain.Grants(at(mine, g.CompanyID, 0), g.Permission) {
			return false
		}
	}
	return true
}

func (s *Service) CreateRole(ctx context.Context, actor int64, in RoleInput) (domain.Role, error) {
	in, err := s.clean(in)
	if err != nil {
		return domain.Role{}, err
	}
	if ok, err := s.mayEditRole(ctx, actor, 0, in.Permissions); err != nil || !ok {
		return domain.Role{}, beyondOwn(err)
	}
	id, err := s.repo.CreateRole(ctx, in.Name, in.Description, in.Permissions)
	if err != nil {
		return domain.Role{}, nameTaken(err)
	}
	s.changed(ctx)
	s.audit.Record(ctx, kernel.AuditEntry{Module: "rbac", Action: "role.created", Target: roleTarget(id), Summary: "Membuat peran " + in.Name,
		Data: map[string]any{"permissions": in.Permissions}})
	return s.GetRole(ctx, id)
}

// UpdateRole changes a role. The role's row stays locked from the check to the write, so nobody
// gives it to someone in another company in between — the check covers every company it reaches.
func (s *Service) UpdateRole(ctx context.Context, actor, id int64, in RoleInput) (domain.Role, error) {
	in, err := s.clean(in)
	if err != nil {
		return domain.Role{}, err
	}
	var current domain.Role
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.LockRole(ctx, id, true); err != nil {
			return notFound(err)
		}
		var err error
		if current, err = s.GetRole(ctx, id); err != nil {
			return err
		}
		if current.IsSystem {
			return apperror.Conflict("Peran Administrator selalu memegang semua izin dan tidak dapat diubah")
		}
		if ok, err := s.mayEditRole(ctx, actor, id, append(rolePerms(current), in.Permissions...)); err != nil || !ok {
			return beyondOwn(err)
		}
		if err := s.repo.UpdateRole(ctx, id, in.Name, in.Description, in.Permissions); err != nil {
			return nameTaken(notFound(err))
		}
		s.changed(ctx)
		return nil
	})
	if err != nil {
		return domain.Role{}, err
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "rbac", Action: "role.updated", Target: roleTarget(id), Summary: "Mengubah peran " + in.Name,
		Data: map[string]any{"name": map[string]any{"before": current.Name, "after": in.Name}, "permissions": map[string]any{"before": current.Permissions, "after": in.Permissions}}})
	return s.GetRole(ctx, id)
}

func (s *Service) DeleteRole(ctx context.Context, actor, id int64) error {
	var r domain.Role
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.repo.LockRole(ctx, id, true); err != nil {
			return notFound(err)
		}
		var err error
		if r, err = s.GetRole(ctx, id); err != nil {
			return err
		}
		if r.IsSystem {
			return apperror.Conflict("Peran Administrator tidak dapat dihapus")
		}
		if ok, err := s.mayEditRole(ctx, actor, id, rolePerms(r)); err != nil || !ok {
			return beyondOwn(err)
		}
		if err := s.repo.DeleteRole(ctx, id); err != nil {
			return notFound(err)
		}
		s.changed(ctx)
		return nil
	})
	if err != nil {
		return err
	}
	s.audit.Record(ctx, kernel.AuditEntry{Module: "rbac", Action: "role.deleted", Target: roleTarget(id), Summary: "Menghapus peran " + r.Name,
		Data: map[string]any{"permissions": r.Permissions}})
	return nil
}

// Members returns who holds the role in the active company, and where.
func (s *Service) Members(ctx context.Context, roleID int64) ([]Member, error) {
	if _, err := s.GetRole(ctx, roleID); err != nil {
		return nil, err
	}
	assigned, err := s.repo.Members(ctx, company(ctx), roleID)
	if err != nil {
		return nil, err
	}
	out := make([]Member, 0, len(assigned))
	for _, a := range assigned {
		u, err := s.users.Get(ctx, a.UserID)
		switch {
		case errors.Is(err, ports.ErrUserNotFound):
			out = append(out, Member{User: ports.User{ID: a.UserID}, BranchID: a.BranchID, Missing: true})
		case err != nil:
			return nil, err
		default:
			out = append(out, Member{User: u, BranchID: a.BranchID})
		}
	}
	return out, nil
}

// where names an assignment's place for the audit trail.
func where(branch int64) string {
	if branch == 0 {
		return "seluruh perusahaan"
	}
	return fmt.Sprintf("cabang #%d", branch)
}

// AddMember gives the role to a user the directory knows, in the active company — everywhere in
// it (branch 0) or in one branch. Only by someone holding rbac.manage and the role's
// permissions there, so nobody can make themselves (or a friend) Administrator. That branch
// exists is for the organization module to say; rbac takes the number as given.
func (s *Service) AddMember(ctx context.Context, actor, roleID, userID, branch int64) error {
	if branch < 0 {
		return apperror.BadRequest("Cabang tidak valid")
	}
	if _, err := s.users.Get(ctx, userID); errors.Is(err, ports.ErrUserNotFound) {
		return apperror.NotFound("Pengguna tidak ditemukan")
	} else if err != nil {
		return err
	}
	// Only someone placed there: a role elsewhere would also shield the account from its own
	// company's administrators (Covers).
	unit := fmt.Sprint(company(ctx))
	if branch != 0 {
		unit += fmt.Sprintf(":%d", branch)
	}
	sc, err := s.scopes.Resolve(ctx, userID, unit)
	if errors.Is(err, kernel.ErrScopeDenied) || (err == nil && (sc.CompanyID != company(ctx) || sc.BranchID != branch)) {
		return apperror.BadRequest("Pengguna tidak ditempatkan di " + where(branch) + " ini")
	}
	if err != nil {
		return err
	}
	var r domain.Role
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		// Shared: the role cannot change between the check and the grant.
		if err := s.repo.LockRole(ctx, roleID, false); err != nil {
			return notFound(err)
		}
		var err error
		if r, err = s.GetRole(ctx, roleID); err != nil {
			return err
		}
		if !s.holds(ctx, actor, company(ctx), branch, append([]string{domain.PermManage}, rolePerms(r)...)...) {
			return errBeyondOwn
		}
		if err := s.repo.AddMember(ctx, company(ctx), branch, roleID, userID); err != nil {
			return err
		}
		s.changed(ctx)
		return nil
	})
	if err != nil {
		return err
	}
	s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "rbac", Action: "role.member_added", Target: fmt.Sprintf("user:%d", userID),
		Summary: "Memberi peran " + r.Name + " di " + where(branch), Data: map[string]any{"role_id": roleID, "role": r.Name, "branch_id": branch}})
	return nil
}

// RemoveMember takes the role away there. The company's last active company-wide
// Administrator cannot lose it — someone must always be able to manage roles. The check and the
// removal run in one transaction holding the role's rows, so two removals at once cannot both
// pass it.
func (s *Service) RemoveMember(ctx context.Context, actor, roleID, userID, branch int64) error {
	var r domain.Role
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		// Shared, as in AddMember: the role cannot gain a permission between the check and the
		// removal. The role's row comes before user_roles, as everywhere.
		if err := s.repo.LockRole(ctx, roleID, false); err != nil {
			return notFound(err)
		}
		var err error
		if r, err = s.GetRole(ctx, roleID); err != nil {
			return err
		}
		if !s.holds(ctx, actor, company(ctx), branch, append([]string{domain.PermManage}, rolePerms(r)...)...) {
			return errBeyondOwn
		}
		if r.IsSystem && branch == 0 {
			assigned, err := s.repo.LockMembers(ctx, company(ctx), roleID)
			if err != nil {
				return err
			}
			left := 0
			for _, a := range assigned {
				if a.BranchID != 0 || a.UserID == userID {
					continue
				}
				u, err := s.users.Get(ctx, a.UserID)
				if err != nil && !errors.Is(err, ports.ErrUserNotFound) {
					return err
				}
				if err == nil && u.IsActive {
					left++
				}
			}
			if left == 0 {
				return apperror.Conflict("Minimal satu pengguna aktif harus tetap memegang peran Administrator di seluruh perusahaan")
			}
		}
		if err := s.repo.RemoveMember(ctx, company(ctx), branch, roleID, userID); err != nil {
			return notFound(err)
		}
		s.changed(ctx)
		return nil
	})
	if err != nil {
		return err
	}
	s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "rbac", Action: "role.member_removed", Target: fmt.Sprintf("user:%d", userID),
		Summary: "Mencabut peran " + r.Name + " di " + where(branch), Data: map[string]any{"role_id": roleID, "role": r.Name, "branch_id": branch}})
	return nil
}

// SearchUsers finds users to give a role to.
func (s *Service) SearchUsers(ctx context.Context, q string) ([]ports.User, error) {
	return s.users.Search(ctx, strings.TrimSpace(q), 10)
}

// grants returns what the user holds in every company, cached for a short time; every change to
// roles or members clears the cache.
func (s *Service) grants(ctx context.Context, userID int64) ([]Grant, error) {
	key := grantKey{user: userID}
	s.mu.Lock()
	c, ok := s.cache[key]
	gen := s.gen
	s.mu.Unlock()
	if ok && s.now().Before(c.until) {
		return c.grants, nil
	}
	grants, err := s.repo.GrantsOfUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.gen == gen && !s.tx.InTx(ctx) { // read before a change, or inside a transaction: not cached
		s.cache[key] = cached{grants: grants, until: s.now().Add(s.ttl)}
	}
	s.mu.Unlock()
	return grants, nil
}

// Granted returns the user's permissions where the request works: in its branch, the company's
// grants plus that branch's; at the whole company, company-wide grants only — someone holding a
// role in one branch works in that branch's scope.
func (s *Service) Granted(ctx context.Context, userID int64) (map[string]bool, error) {
	grants, err := s.grants(ctx, userID)
	if err != nil {
		return nil, err
	}
	sc := kernel.ScopeFrom(ctx)
	return at(grants, sc.CompanyID, sc.BranchID), nil
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

// Units is which branches of the active company permission reaches for the user (the kernel's
// ScopedAuthorizer): every branch when held company-wide, else the branches it is held in; in
// one branch's scope, that branch or nothing.
func (s *Service) Units(ctx context.Context, userID int64, permission string) (kernel.Units, error) {
	grants, err := s.grants(ctx, userID)
	if err != nil {
		return kernel.Units{}, err
	}
	sc := kernel.ScopeFrom(ctx)
	if sc.BranchID != 0 {
		if domain.Grants(at(grants, sc.CompanyID, sc.BranchID), permission) {
			return kernel.Units{BranchIDs: []int64{sc.BranchID}}, nil
		}
		return kernel.Units{}, nil
	}
	if domain.Grants(at(grants, sc.CompanyID, 0), permission) {
		return kernel.Units{AllBranches: true}, nil
	}
	var u kernel.Units
	seen := map[int64]bool{}
	for _, g := range grants {
		if g.CompanyID == sc.CompanyID && g.BranchID != 0 && !seen[g.BranchID] && (g.Permission == permission || g.Permission == domain.Wildcard) {
			seen[g.BranchID] = true
			u.BranchIDs = append(u.BranchIDs, g.BranchID)
		}
	}
	sort.Slice(u.BranchIDs, func(i, j int) bool { return u.BranchIDs[i] < u.BranchIDs[j] })
	return u, nil
}

// Bootstrap gives the Administrator role to the user with email when no active user holds it
// — how a fresh install gets someone who can manage roles, and how an install whose only
// administrators were deactivated or removed gets one back.
func (s *Service) Bootstrap(ctx context.Context, email string) (assigned bool, err error) {
	adminID, err := s.repo.RoleIDByName(ctx, domain.AdminRole)
	if err != nil {
		return false, err
	}
	admins, err := s.repo.Members(ctx, kernel.DefaultCompany, adminID)
	if err != nil {
		return false, err
	}
	for _, a := range admins {
		if a.BranchID != 0 {
			continue
		}
		u, err := s.users.Get(ctx, a.UserID)
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
	if err := s.repo.AddMember(ctx, kernel.DefaultCompany, 0, adminID, u.ID); err != nil {
		return false, err
	}
	s.invalidate()
	return true, nil
}

// changed clears the cache now and again once the change commits — a lookup in between would
// otherwise cache what is about to be replaced.
func (s *Service) changed(ctx context.Context) {
	s.invalidate()
	s.tx.AfterCommit(ctx, func(context.Context) { s.invalidate() })
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cache = map[grantKey]cached{}
	s.gen++
	s.mu.Unlock()
}

// beyondOwn is errBeyondOwn, or the lookup error that kept the check from answering.
func beyondOwn(err error) error {
	if err != nil {
		return err
	}
	return errBeyondOwn
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
