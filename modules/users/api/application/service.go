// Package application holds the users module's use cases.
package application

import (
	"context"
	"errors"
	"time"

	"__GO_MODULE__/internal/modules/users/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/validator"
)

// ErrNotFound and ErrEmailTaken are what a Repository reports.
var (
	ErrNotFound   = errors.New("user not found")
	ErrEmailTaken = errors.New("email already in use")
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
	CountAll(ctx context.Context) (int64, error)
	Get(ctx context.Context, id int64) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetCredentialsByEmail(ctx context.Context, email string) (Credentials, error)
	Create(ctx context.Context, u NewUser) (int64, error)
	Update(ctx context.Context, u domain.User) error
	UpdatePassword(ctx context.Context, id int64, hash string, at time.Time) error
}

// Guard answers whether the signed-in user may act on another user's account — the kernel's
// d.Authz satisfies it: an account holding permissions the actor lacks is off limits.
type Guard interface {
	Covers(ctx context.Context, target int64) bool
}

// Hasher turns a password into a stored hash.
type Hasher interface {
	Hash(password string) (string, error)
}

// Service is the users module's use cases.
type Service struct {
	repo   Repository
	hasher Hasher
	guard  Guard
}

func NewService(repo Repository, hasher Hasher, guard Guard) *Service {
	return &Service{repo: repo, hasher: hasher, guard: guard}
}

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

func (s *Service) List(ctx context.Context, search string, p pagination.Params) ([]domain.User, int64, error) {
	return s.repo.List(ctx, search, p)
}

func (s *Service) Get(ctx context.Context, id int64) (domain.User, error) {
	u, err := s.repo.Get(ctx, id)
	return u, notFound(err)
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
	id, err := s.repo.Create(ctx, NewUser{Name: in.Name, Email: in.Email, PasswordHash: hash, IsActive: in.IsActive})
	if err != nil {
		return domain.User{}, emailTaken(err)
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
	u, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.User{}, notFound(err)
	}
	// Someone else's account: only when it holds no permission the actor lacks — an operator
	// cannot reset an administrator's password, change their email, or deactivate them.
	if id != actorID && !s.guard.Covers(ctx, id) {
		return domain.User{}, apperror.Forbidden("Anda tidak dapat mengubah pengguna yang memiliki hak lebih tinggi")
	}
	u.Name, u.Email = in.Name, in.Email
	if in.IsActive != nil {
		u.IsActive = *in.IsActive
	}
	if err := s.repo.Update(ctx, u); err != nil {
		return domain.User{}, emailTaken(notFound(err))
	}
	if in.Password != "" {
		hash, err := s.hasher.Hash(in.Password)
		if err != nil {
			return domain.User{}, err
		}
		// Recording when it changed ends every session opened before (the auth provider checks).
		if err := s.repo.UpdatePassword(ctx, id, hash, time.Now().UTC()); err != nil {
			return domain.User{}, err
		}
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
