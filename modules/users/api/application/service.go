// Package application holds the users module's use cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/users/domain"
	"__GO_MODULE__/internal/modules/users/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/validator"
)

// ErrNotFound and ErrEmailTaken are what a Repository reports; ErrInvalidPassword is a new
// password that breaks the rules.
var (
	ErrNotFound        = errors.New("user not found")
	ErrEmailTaken      = errors.New("email already in use")
	ErrInvalidPassword = errors.New("invalid password")
)

// Credentials is a stored password hash and whether the account may sign in.
type Credentials struct {
	UserID       int64
	PasswordHash string
	IsActive     bool
}

// NewUser is a user to create, with the password already hashed.
type NewUser struct {
	Name         string
	Email        string
	PasswordHash string
	IsActive     bool
}

// Repository is the users table.
type Repository interface {
	List(ctx context.Context, search string, p pagination.Params) ([]domain.User, int64, error)
	// ListIn is List among ids only.
	ListIn(ctx context.Context, ids []int64, search string, p pagination.Params) ([]domain.User, int64, error)
	IDs(ctx context.Context) ([]int64, error)
	CountAll(ctx context.Context) (int64, error)
	Get(ctx context.Context, id int64) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetCredentialsByEmail(ctx context.Context, email string) (Credentials, error)
	Create(ctx context.Context, u NewUser) (int64, error)
	Update(ctx context.Context, u domain.User) error
	// UpdateName changes the name alone, so it never writes back anything else it did not read
	// under a lock (an admin's deactivation or new email in the meantime).
	UpdateName(ctx context.Context, id int64, name string) error
	UpdatePassword(ctx context.Context, id int64, hash string, at time.Time) error
}

// Auditor records changes — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// Guard answers whether the signed-in user may act on another user's account — the kernel's
// d.Authz satisfies it: an account holding permissions the actor lacks is off limits. Can is
// asked in a given scope (the account's other workplaces).
type Guard interface {
	Covers(ctx context.Context, target int64) bool
	Can(ctx context.Context, permission string) bool
}

// permManage is the permission the users pages change accounts with (presentation.PermManage).
const permManage = "users.manage"

// Transactor runs fn in one transaction — the kernel's d.Tx.
type Transactor interface {
	Do(ctx context.Context, fn func(ctx context.Context) error, opts ...kernel.TxOption) error
}

// Hasher turns a password into a stored hash.
type Hasher interface {
	Hash(password string) (string, error)
}

// Service is the users module's use cases.
type Service struct {
	repo       Repository
	tx         Transactor
	hasher     Hasher
	guard      Guard
	audit      Auditor
	placements ports.Placements // nil: one company, everyone visible
}

func NewService(repo Repository, tx Transactor, hasher Hasher, guard Guard, audit Auditor) *Service {
	return &Service{repo: repo, tx: tx, hasher: hasher, guard: guard, audit: audit}
}

// UsePlacements turns on per-company lists — set once, while the app is assembled.
func (s *Service) UsePlacements(p ports.Placements) { s.placements = p }

func target(id int64) string { return fmt.Sprintf("user:%d", id) }

// CreateInput is what creating a user takes.
type CreateInput struct {
	Name     string
	Email    string
	Password string
	IsActive bool
}

// UpdateInput is what changing a user takes. An empty Password keeps the current one; a nil
// IsActive keeps the current state.
type UpdateInput struct {
	Name     string
	Email    string
	Password string
	IsActive *bool
}

// List is every user — for the contract (an auth provider, an organization module placing
// people). The users pages use ListHere.
func (s *Service) List(ctx context.Context, search string, p pagination.Params) ([]domain.User, int64, error) {
	return s.repo.List(ctx, search, p)
}

// ListHere is the users pages' list: everyone, or — with placements — the people placed in the
// company the request works in.
func (s *Service) ListHere(ctx context.Context, search string, p pagination.Params) ([]domain.User, int64, error) {
	if s.placements == nil {
		return s.repo.List(ctx, search, p)
	}
	ids, err := s.placements.PlacedUserIDs(ctx, kernel.ScopeFrom(ctx).CompanyID)
	if err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return []domain.User{}, 0, nil
	}
	return s.repo.ListIn(ctx, ids, search, p)
}

// ListAmong is List among ids — for the contract (a search within one company's people).
func (s *Service) ListAmong(ctx context.Context, ids []int64, search string, p pagination.Params) ([]domain.User, int64, error) {
	return s.repo.ListIn(ctx, ids, search, p)
}

// IDs is every user's ID — for the contract (an organization module placing everyone at
// install).
func (s *Service) IDs(ctx context.Context) ([]int64, error) { return s.repo.IDs(ctx) }

func (s *Service) Get(ctx context.Context, id int64) (domain.User, error) {
	u, err := s.repo.Get(ctx, id)
	return u, notFound(err)
}

// GetHere is Get for the users pages: someone not placed in the company the request works in
// is not found here.
func (s *Service) GetHere(ctx context.Context, id int64) (domain.User, error) {
	if err := s.here(ctx, id); err != nil {
		return domain.User{}, err
	}
	return s.Get(ctx, id)
}

// managesEverywhere refuses changing an account that is also placed somewhere — active or not —
// the actor does not manage users: another company, another branch. So no company takes over
// another's account, and deactivating a unit hands its people to no one.
func (s *Service) managesEverywhere(ctx context.Context, id int64) error {
	if s.placements == nil {
		return nil
	}
	places, err := s.placements.PlacesOf(ctx, id)
	if err != nil {
		return err
	}
	if len(places) == 0 {
		places = []ports.Workplace{{CompanyID: kernel.DefaultCompany}} // placed nowhere: the home company's
	}
	for _, p := range places {
		if !s.guard.Can(kernel.WithScope(ctx, kernel.Scope{CompanyID: p.CompanyID, BranchID: p.BranchID}), permManage) {
			return apperror.Forbidden("Pengguna ini juga bekerja di tempat yang tidak Anda kelola")
		}
	}
	return nil
}

// here refuses a user not placed in the request's company (with placements wired).
func (s *Service) here(ctx context.Context, id int64) error {
	if s.placements == nil {
		return nil
	}
	ok, err := s.placements.IsPlaced(ctx, id, kernel.ScopeFrom(ctx).CompanyID)
	if err != nil {
		return err
	}
	if !ok {
		return apperror.NotFound("Pengguna tidak ditemukan")
	}
	return nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (domain.User, error) {
	in.Email = domain.NormalizeEmail(in.Email)
	var v validator.Validator
	domain.ValidateProfile(&v, in.Name, in.Email)
	domain.ValidatePassword(&v, "password", in.Password)
	if err := v.Err(); err != nil {
		return domain.User{}, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return domain.User{}, err
	}
	// The account, its placement where it was created (with placements wired), and the audit
	// record commit together.
	var id int64
	err = s.tx.Do(ctx, func(ctx context.Context) error {
		var err error
		if id, err = s.repo.Create(ctx, NewUser{Name: in.Name, Email: in.Email, PasswordHash: hash, IsActive: in.IsActive}); err != nil {
			return emailTaken(err)
		}
		if s.placements != nil {
			if err := s.placements.PlaceNew(ctx, id); err != nil {
				return err
			}
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "users", Action: "user.created", Target: target(id), Summary: "Menambah pengguna " + in.Name,
			Data: map[string]any{"name": in.Name, "email": in.Email, "is_active": in.IsActive}})
		return nil
	})
	if err != nil {
		return domain.User{}, err
	}
	return s.repo.Get(ctx, id)
}

// Update changes a user. actorID is the signed-in user: nobody can deactivate themselves.
func (s *Service) Update(ctx context.Context, actorID, id int64, in UpdateInput) (domain.User, error) {
	in.Email = domain.NormalizeEmail(in.Email)
	var v validator.Validator
	domain.ValidateProfile(&v, in.Name, in.Email)
	if in.Password != "" {
		domain.ValidatePassword(&v, "password", in.Password)
	}
	if err := v.Err(); err != nil {
		return domain.User{}, err
	}
	if id == actorID && in.IsActive != nil && !*in.IsActive {
		return domain.User{}, apperror.Conflict("Anda tidak dapat menonaktifkan akun sendiri")
	}
	if id != actorID {
		if err := s.here(ctx, id); err != nil {
			return domain.User{}, err
		}
	}
	u, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.User{}, notFound(err)
	}
	// Someone else's account: only when it holds no permission the actor lacks — an operator
	// cannot reset an administrator's password, change their email, or deactivate them — and,
	// with placements, only by someone managing users everywhere the account works.
	if id != actorID && !s.guard.Covers(ctx, id) {
		return domain.User{}, apperror.Forbidden("Anda tidak dapat mengubah pengguna yang memiliki hak lebih tinggi")
	}
	if id != actorID {
		if err := s.managesEverywhere(ctx, id); err != nil {
			return domain.User{}, err
		}
	}
	before := u
	u.Name, u.Email = in.Name, in.Email
	if in.IsActive != nil {
		u.IsActive = *in.IsActive
	}
	if err := s.repo.Update(ctx, u); err != nil {
		return domain.User{}, emailTaken(notFound(err))
	}
	if in.Password != "" {
		if err := s.setPassword(ctx, id, in.Password); err != nil {
			return domain.User{}, err
		}
	}
	s.recordUpdate(ctx, before, u, in.Password != "")
	return s.repo.Get(ctx, id)
}

// recordUpdate notes what an update changed — the active state in words of its own.
func (s *Service) recordUpdate(ctx context.Context, before, after domain.User, password bool) {
	changes := map[string]any{}
	if before.Name != after.Name {
		changes["name"] = map[string]any{"before": before.Name, "after": after.Name}
	}
	if before.Email != after.Email {
		changes["email"] = map[string]any{"before": before.Email, "after": after.Email}
	}
	if password {
		changes["password"] = "diganti"
	}
	if before.IsActive != after.IsActive {
		action, summary := "user.deactivated", "Menonaktifkan "+after.Name
		if after.IsActive {
			action, summary = "user.activated", "Mengaktifkan "+after.Name
		}
		s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditSecurity, Module: "users", Action: action, Target: target(after.ID), Summary: summary})
	}
	if len(changes) > 0 {
		kind := kernel.AuditChange
		if password || changes["email"] != nil {
			kind = kernel.AuditSecurity // a new password or sign-in address is how an account changes hands
		}
		s.audit.Record(ctx, kernel.AuditEntry{Kind: kind, Module: "users", Action: "user.updated", Target: target(after.ID), Summary: "Mengubah pengguna " + after.Name, Data: changes})
	}
}

func (s *Service) setPassword(ctx context.Context, id int64, password string) error {
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	// Recording when it changed ends every session opened before (the auth provider checks).
	return s.repo.UpdatePassword(ctx, id, hash, time.Now().UTC())
}

// SetPassword replaces a user's password for an auth provider (the person proved who they
// are there). An invalid password is ErrInvalidPassword, followed by the rule it breaks.
func (s *Service) SetPassword(ctx context.Context, id int64, password string) error {
	var v validator.Validator
	domain.ValidatePassword(&v, "password", password)
	if err := v.Err(); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidPassword, firstProblem(err))
	}
	if _, err := s.repo.Get(ctx, id); err != nil {
		return err
	}
	return s.setPassword(ctx, id, password)
}

// UpdateProfile changes the signed-in user's own name. Email stays with administrators:
// changing it oneself would be a way to take an account over.
func (s *Service) UpdateProfile(ctx context.Context, id int64, name string) (domain.User, error) {
	var v validator.Validator
	v.Required("name", name)
	v.MaxLen("name", name, domain.MaxNameLength)
	if err := v.Err(); err != nil {
		return domain.User{}, err
	}
	u, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.User{}, notFound(err)
	}
	if err := s.repo.UpdateName(ctx, id, name); err != nil {
		return domain.User{}, notFound(err)
	}
	if u.Name != name {
		s.audit.Record(ctx, kernel.AuditEntry{Module: "users", Action: "user.profile_updated", Target: target(id), Summary: "Mengubah profil sendiri",
			Data: map[string]any{"name": map[string]any{"before": u.Name, "after": name}}})
	}
	return s.repo.Get(ctx, id)
}

// Bootstrap creates the first user when the table is empty — how a fresh install gets an
// account to sign in with. It does nothing once any user exists.
func (s *Service) Bootstrap(ctx context.Context, name, email, password string) (created bool, err error) {
	n, err := s.repo.CountAll(ctx)
	if err != nil || n > 0 {
		return false, err
	}
	if _, err := s.Create(ctx, CreateInput{Name: name, Email: email, Password: password, IsActive: true}); err != nil {
		return false, err
	}
	return true, nil
}

// GetByEmail is for the contract.
func (s *Service) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := s.repo.GetByEmail(ctx, domain.NormalizeEmail(email))
	return u, notFound(err)
}

// GetCredentialsByEmail is for the contract (an auth provider signing someone in).
func (s *Service) GetCredentialsByEmail(ctx context.Context, email string) (Credentials, error) {
	return s.repo.GetCredentialsByEmail(ctx, domain.NormalizeEmail(email))
}

func notFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("Pengguna tidak ditemukan")
	}
	return err
}

func emailTaken(err error) error {
	if errors.Is(err, ErrEmailTaken) {
		return apperror.Validation([]validator.FieldError{{Field: "email", Message: "sudah dipakai"}})
	}
	return err
}

// firstProblem is the first field message of a validation error, in words for the person.
func firstProblem(err error) string {
	if e, ok := apperror.As(err); ok {
		if fields, ok := e.Details.([]validator.FieldError); ok && len(fields) > 0 {
			return fields[0].Message
		}
	}
	return err.Error()
}
