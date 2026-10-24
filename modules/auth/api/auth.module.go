// Package auth signs users in with email and password and guards every module's routes by
// filling the kernel's d.Auth. It needs one port — ports.UserStore — and imports no module.
package auth

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"__GO_MODULE__/internal/config"
	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/auth/application"
	"__GO_MODULE__/internal/modules/auth/ports"
	"__GO_MODULE__/internal/modules/auth/presentation"
)

// Module is the auth module.
type Module struct {
	handler *presentation.Handler
}

// New builds the module. JWT_SECRET is required (at least 32 characters in production);
// JWT_EXPIRES_IN defaults to 24h.
func New(d *kernel.Deps, users ports.UserStore) (*Module, error) {
	secret, err := config.Require("JWT_SECRET")
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	if d.Config.IsProduction() && (len(secret) < 32 || secret == "change-me") {
		return nil, errors.New("auth: JWT_SECRET must be a random value of at least 32 characters in production (openssl rand -hex 32)")
	}
	ttl, err := time.ParseDuration(config.Get("JWT_EXPIRES_IN", "24h"))
	if err != nil || ttl <= 0 {
		return nil, fmt.Errorf("auth: JWT_EXPIRES_IN must be a duration such as 24h (got %q)", config.Get("JWT_EXPIRES_IN", ""))
	}
	svc := application.NewService(users, application.NewTokens(secret, ttl), application.NewLimiter(5, 15*time.Minute))
	return &Module{handler: presentation.NewHandler(svc, d.Config.IsProduction())}, nil
}

// Routes mounts /api/v1/auth.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Authenticator is what the composition root passes to d.Auth.Provide.
func (m *Module) Authenticator() kernel.Authenticator { return m.handler }
