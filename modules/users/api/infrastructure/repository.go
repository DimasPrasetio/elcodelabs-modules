// Package infrastructure stores users in MySQL through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/users/application"
	"__GO_MODULE__/internal/modules/users/domain"
	"__GO_MODULE__/internal/modules/users/infrastructure/sqlc"
	"__GO_MODULE__/internal/shared/pagination"
)

// Repository implements application.Repository.
type Repository struct {
	tx *kernel.TxSlot
}

func NewRepository(tx *kernel.TxSlot) *Repository {
	return &Repository{tx: tx}
}

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func (r *Repository) List(ctx context.Context, search string, p pagination.Params) ([]domain.User, int64, error) {
	rows, err := r.q(ctx).ListUsers(ctx, sqlc.ListUsersParams{Search: search, Limit: int32(p.Limit), Offset: int32(p.Offset())})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q(ctx).CountUsers(ctx, sqlc.CountUsersParams{Search: search})
	if err != nil {
		return nil, 0, err
	}
	users := make([]domain.User, len(rows))
	for i, row := range rows {
		users[i] = domain.User{ID: row.ID, Name: row.Name, Email: row.Email, IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	}
	return users, total, nil
}

func (r *Repository) ListIn(ctx context.Context, ids []int64, search string, p pagination.Params) ([]domain.User, int64, error) {
	rows, err := r.q(ctx).ListUsersIn(ctx, sqlc.ListUsersInParams{Ids: ids, Search: search, Limit: int32(p.Limit), Offset: int32(p.Offset())})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q(ctx).CountUsersIn(ctx, sqlc.CountUsersInParams{Ids: ids, Search: search})
	if err != nil {
		return nil, 0, err
	}
	users := make([]domain.User, len(rows))
	for i, row := range rows {
		users[i] = domain.User{ID: row.ID, Name: row.Name, Email: row.Email, IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	}
	return users, total, nil
}

func (r *Repository) IDs(ctx context.Context) ([]int64, error) { return r.q(ctx).UserIDs(ctx) }

func (r *Repository) CountAll(ctx context.Context) (int64, error) {
	return r.q(ctx).CountAllUsers(ctx)
}

func (r *Repository) Get(ctx context.Context, id int64) (domain.User, error) {
	row, err := r.q(ctx).GetUser(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, application.ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return domain.User{ID: row.ID, Name: row.Name, Email: row.Email, IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, PasswordChangedAt: row.PasswordChangedAt.Time}, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	row, err := r.q(ctx).GetUserByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, application.ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return domain.User{ID: row.ID, Name: row.Name, Email: row.Email, IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func (r *Repository) GetCredentialsByEmail(ctx context.Context, email string) (application.Credentials, error) {
	row, err := r.q(ctx).GetCredentialsByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return application.Credentials{}, application.ErrNotFound
	}
	if err != nil {
		return application.Credentials{}, err
	}
	return application.Credentials{UserID: row.ID, PasswordHash: row.PasswordHash, IsActive: row.IsActive}, nil
}

func (r *Repository) Create(ctx context.Context, u application.NewUser) (int64, error) {
	id, err := r.q(ctx).CreateUser(ctx, sqlc.CreateUserParams{Name: u.Name, Email: u.Email, PasswordHash: u.PasswordHash, IsActive: u.IsActive})
	return id, duplicate(err)
}

func (r *Repository) UpdateName(ctx context.Context, id int64, name string) error {
	n, err := r.q(ctx).UpdateUserName(ctx, sqlc.UpdateUserNameParams{Name: name, ID: id})
	if err != nil {
		return err
	}
	if n == 0 {
		if _, err := r.Get(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, u domain.User) error {
	n, err := r.q(ctx).UpdateUser(ctx, sqlc.UpdateUserParams{Name: u.Name, Email: u.Email, IsActive: u.IsActive, ID: u.ID})
	if err != nil {
		return duplicate(err)
	}
	if n == 0 {
		// MySQL reports 0 affected rows when nothing changed, too — tell the two apart.
		if _, err := r.Get(ctx, u.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) UpdatePassword(ctx context.Context, id int64, hash string, at time.Time) error {
	return r.q(ctx).UpdatePassword(ctx, sqlc.UpdatePasswordParams{PasswordHash: hash, PasswordChangedAt: sql.NullTime{Time: at, Valid: true}, ID: id})
}

// duplicate maps MySQL's duplicate-key error on the email index to ErrEmailTaken.
func duplicate(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return application.ErrEmailTaken
	}
	return err
}

// BcryptHasher implements application.Hasher.
type BcryptHasher struct{}

func (BcryptHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}
