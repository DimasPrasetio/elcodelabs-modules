// Package rbac decides who may do what: roles hold permissions that modules declared
// (d.Authz.Declare), users hold roles. It fills the kernel's d.Authz, needs one port —
// ports.UserDirectory — and imports no module.
package rbac

import (
	"context"
	"net/http"
	"time"

	"__GO_MODULE__/internal/config"
	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/rbac/application"
	"__GO_MODULE__/internal/modules/rbac/infrastructure"
	"__GO_MODULE__/internal/modules/rbac/ports"
	"__GO_MODULE__/internal/modules/rbac/presentation"
)

// Module is the rbac module.
type Module struct {
	svc     *application.Service
	auth    kernel.Authenticator
	handler *presentation.Handler
}

// New builds the module. When no active user holds the Administrator role, it gives it to the user
// with RBAC_BOOTSTRAP_ADMIN_EMAIL (default: USERS_BOOTSTRAP_EMAIL) — so installing rbac never
// locks everyone out. A failed bootstrap is logged, not fatal; the next start tries again.
func New(d *kernel.Deps, users ports.UserDirectory) (*Module, error) {
	d.Authz.Declare(kernel.Permission{Name: presentation.PermManage, Module: "rbac", Description: "Mengelola peran, izinnya, dan pemegangnya"})
	svc := application.NewService(infrastructure.NewRepository(d.DB), users, d.Authz.Permissions)
	m := &Module{svc: svc, auth: d.Auth, handler: presentation.NewHandler(svc, d.Auth, d.Authz)}

	email := config.Get("RBAC_BOOTSTRAP_ADMIN_EMAIL", config.Get("USERS_BOOTSTRAP_EMAIL", ""))
	if email == "" {
		return m, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assigned, err := svc.Bootstrap(ctx, email)
	switch {
	case err != nil:
		d.Log.Warn("rbac: Administrator not bootstrapped", "email", email, "err", err)
	case assigned:
		d.Log.Info("rbac: Administrator role given", "email", email)
	}
	return m, nil
}

// Routes mounts /api/v1/rbac.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Authorizer is what the composition root passes to d.Authz.Provide.
func (m *Module) Authorizer() kernel.Authorizer { return authorizer{m} }

type authorizer struct{ m *Module }

func (a authorizer) Can(ctx context.Context, permission string) bool {
	id, ok := a.m.auth.UserID(ctx)
	return ok && a.m.svc.Can(ctx, id, permission)
}

func (a authorizer) Covers(ctx context.Context, target int64) bool {
	id, ok := a.m.auth.UserID(ctx)
	return ok && a.m.svc.Covers(ctx, id, target)
}
