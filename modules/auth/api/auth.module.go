// Package auth signs users in with email and password and guards every module's routes by
// filling the kernel's d.Auth. It needs one port — ports.UserStore — and imports no module.
package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"__GO_MODULE__/internal/config"
	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/auth/application"
	"__GO_MODULE__/internal/modules/auth/infrastructure"
	"__GO_MODULE__/internal/modules/auth/ports"
	"__GO_MODULE__/internal/modules/auth/presentation"
)

// Module is the auth module.
type Module struct {
	handler *presentation.Handler
}

// New builds the module. JWT_SECRET is required (at least 32 characters in production);
// JWT_EXPIRES_IN defaults to 24h. passwords and mailer are optional (nil): without passwords
// there is no "change password" or "forgot password"; without mailer, no "forgot password".
// With both, APP_URL — the address people open the app at — is required in production: reset
// links are built from it, never from a request's Host header.
func New(d *kernel.Deps, users ports.UserStore, passwords ports.PasswordWriter, mailer ports.Mailer) (*Module, error) {
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
	appURL := config.Get("APP_URL", "")
	if passwords != nil && mailer != nil {
		if appURL == "" && d.Config.IsProduction() {
			return nil, errors.New("auth: APP_URL is not set — password reset links need the app's public address (e.g. https://app.tokomaju.com)")
		}
		if appURL == "" {
			appURL = "http://localhost:5173"
		}
		if u, err := url.Parse(appURL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || (d.Config.IsProduction() && u.Scheme != "https") {
			return nil, fmt.Errorf("auth: APP_URL must be the app's address, https in production (got %q)", appURL)
		}
	}
	svc := application.NewService(application.Deps{
		Users: users, Passwords: passwords, Mailer: mailer,
		Sessions: infrastructure.NewSessions(d.DB), Resets: infrastructure.NewResets(d.DB),
		Tokens:  application.NewTokens(secret, ttl, d.Config.Name),
		Limiter: application.NewLimiter(15*time.Minute, 50_000),
		Audit:   d.Audit, Log: d.Log, AppURL: appURL, AppName: d.Config.Name,
	})
	d.Jobs.Every("auth.cleanup", time.Hour, svc.Cleanup)
	return &Module{handler: presentation.NewHandler(svc, d.Config.IsProduction())}, nil
}

// Routes mounts /api/v1/auth.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Authenticator is what the composition root passes to d.Auth.Provide.
func (m *Module) Authenticator() kernel.Authenticator { return m.handler }
