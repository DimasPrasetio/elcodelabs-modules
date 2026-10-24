// Package users manages the people who can sign in. It offers contracts.UserClient and needs
// nothing from other modules; its routes are guarded through the kernel's d.Authz.
package users

import (
	"context"
	"errors"
	"net/http"
	"time"

	"__GO_MODULE__/internal/config"
	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/users/application"
	"__GO_MODULE__/internal/modules/users/contracts"
	"__GO_MODULE__/internal/modules/users/infrastructure"
	"__GO_MODULE__/internal/modules/users/presentation"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/pagination"
)

// Module is the users module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module. When USERS_BOOTSTRAP_EMAIL and USERS_BOOTSTRAP_PASSWORD are set and
// the users table is empty, it creates that first user — how a fresh install gets an
// account. A failed bootstrap (database still down) is logged, not fatal; the next start
// tries again.
func New(d *kernel.Deps) (*Module, error) {
	d.Authz.Declare(
		kernel.Permission{Name: presentation.PermRead, Module: "users", Description: "Melihat daftar pengguna"},
		kernel.Permission{Name: presentation.PermManage, Module: "users", Description: "Menambah, mengubah, menonaktifkan, dan menghapus pengguna"},
	)
	svc := application.NewService(infrastructure.NewRepository(d.DB), infrastructure.BcryptHasher{})
	m := &Module{svc: svc, handler: presentation.NewHandler(svc, d.Auth, d.Authz)}

	email, password := config.Get("USERS_BOOTSTRAP_EMAIL", ""), config.Get("USERS_BOOTSTRAP_PASSWORD", "")
	if email != "" && password != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		name := config.Get("USERS_BOOTSTRAP_NAME", "Administrator")
		created, err := svc.Bootstrap(ctx, name, email, password)
		switch {
		case err != nil:
			d.Log.Warn("users: bootstrap user not created", "err", err)
		case created:
			d.Log.Info("users: bootstrap user created", "email", email)
		}
	}
	return m, nil
}

// Routes mounts /api/v1/users.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Client returns the module's public contract.
func (m *Module) Client() contracts.UserClient { return client{m.svc} }

type client struct{ svc *application.Service }

func (c client) GetByID(ctx context.Context, id int64) (contracts.User, error) {
	u, err := c.svc.Get(ctx, id)
	if err != nil {
		return contracts.User{}, contractErr(err)
	}
	return contracts.User{ID: u.ID, Name: u.Name, Email: u.Email, IsActive: u.IsActive}, nil
}

func (c client) GetByEmail(ctx context.Context, email string) (contracts.User, error) {
	u, err := c.svc.GetByEmail(ctx, email)
	if err != nil {
		return contracts.User{}, contractErr(err)
	}
	return contracts.User{ID: u.ID, Name: u.Name, Email: u.Email, IsActive: u.IsActive}, nil
}

func (c client) Search(ctx context.Context, q string, limit int) ([]contracts.User, error) {
	users, _, err := c.svc.List(ctx, q, pagination.Params{Page: 1, Limit: min(max(limit, 1), pagination.MaxLimit)})
	if err != nil {
		return nil, err
	}
	out := make([]contracts.User, len(users))
	for i, u := range users {
		out[i] = contracts.User{ID: u.ID, Name: u.Name, Email: u.Email, IsActive: u.IsActive}
	}
	return out, nil
}

func (c client) GetCredentialsByEmail(ctx context.Context, email string) (contracts.Credentials, error) {
	cr, err := c.svc.GetCredentialsByEmail(ctx, email)
	if err != nil {
		return contracts.Credentials{}, contractErr(err)
	}
	return contracts.Credentials{UserID: cr.UserID, PasswordHash: cr.PasswordHash, IsActive: cr.IsActive}, nil
}

// contractErr turns "not found" — as a repository error or the 404 the service returns —
// into the contract's ErrNotFound.
func contractErr(err error) error {
	if e, ok := apperror.As(err); (ok && e.Status == http.StatusNotFound) || errors.Is(err, application.ErrNotFound) {
		return contracts.ErrNotFound
	}
	return err
}
