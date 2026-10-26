package kernel

import (
	"context"
	"net/http"
	"sync"

	"testbed-app/internal/shared/response"
)

// Authenticator is what an auth provider offers every module: guarding a handler and telling
// who is signed in. It is a kernel port rather than a per-module one because nearly every
// module needs it — and a module that both guards its routes and is needed by the auth module
// (users) would otherwise depend on auth while auth depends on it.
type Authenticator interface {
	// Require wraps h so it only runs for an authenticated request.
	Require(h http.Handler) http.Handler
	// UserID returns the signed-in user's ID inside a handler that Require let through.
	UserID(ctx context.Context) (int64, bool)
}

// AuthSlot is the Authenticator every module receives in Deps.Auth. The composition root
// fills it with Provide; modules may wrap handlers before that, because the provider is
// looked up per request. Until a provider is set, every guarded request answers 401.
type AuthSlot struct {
	mu sync.RWMutex
	p  Authenticator
}

// Provide sets the auth provider. Called once, from internal/modules/registry.go.
func (s *AuthSlot) Provide(p Authenticator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p = p
}

func (s *AuthSlot) provider() Authenticator {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.p
}

// Require implements Authenticator.
func (s *AuthSlot) Require(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := s.provider()
		if p == nil {
			response.Fail(w, http.StatusUnauthorized, "Autentikasi belum dikonfigurasi", nil)
			return
		}
		p.Require(h).ServeHTTP(w, r)
	})
}

// UserID implements Authenticator.
func (s *AuthSlot) UserID(ctx context.Context) (int64, bool) {
	if p := s.provider(); p != nil {
		return p.UserID(ctx)
	}
	return 0, false
}
