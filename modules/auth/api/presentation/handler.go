// Package presentation is the auth module's HTTP side: sign-in endpoints and the
// kernel.Authenticator every guarded route goes through.
package presentation

import (
	"context"
	"net/http"
	"strings"
	"time"

	"__GO_MODULE__/internal/modules/auth/application"
	"__GO_MODULE__/internal/modules/auth/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
)

// CookieName is the session cookie. HttpOnly: page scripts never see the token.
const CookieName = "session"

type userDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type sessionDTO struct {
	User      userDTO   `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
	// Token is for non-browser clients (Authorization: Bearer). Browsers use the cookie.
	Token string `json:"token"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Handler serves /api/v1/auth and implements kernel.Authenticator.
type Handler struct {
	svc          *application.Service
	secureCookie bool
}

// NewHandler returns the handler. secureCookie marks the cookie HTTPS-only — true in
// production, where nginx terminates TLS.
func NewHandler(svc *application.Service, secureCookie bool) *Handler {
	return &Handler{svc: svc, secureCookie: secureCookie}
}

// Routes mounts the auth endpoints.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	mux.Handle("GET /api/v1/auth/me", h.Require(http.HandlerFunc(h.me)))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	s, err := h.svc.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    s.Token,
		Path:     "/",
		Expires:  s.ExpiresAt,
		HttpOnly: true,
		Secure:   h.secureCookie,
		// Lax: the cookie is not sent on cross-site POSTs, which is the CSRF defense for a
		// same-origin SPA + API.
		SameSite: http.SameSiteLaxMode,
	})
	response.OK(w, "Signed in", sessionDTO{User: toUserDTO(s.User), ExpiresAt: s.ExpiresAt, Token: s.Token})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: h.secureCookie, SameSite: http.SameSiteLaxMode,
	})
	response.OK(w, "Signed out", nil)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, _ := r.Context().Value(userKey{}).(ports.User)
	response.OK(w, "Signed in", toUserDTO(u))
}

type userKey struct{}

// Require implements kernel.Authenticator: the token comes from the session cookie or an
// Authorization: Bearer header, and must belong to an active user.
func (h *Handler) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			if c, err := r.Cookie(CookieName); err == nil {
				token = c.Value
			}
		}
		if token == "" {
			response.Error(w, r, apperror.Unauthorized("Sign in required"))
			return
		}
		u, err := h.svc.Resolve(r.Context(), token)
		if err != nil {
			response.Error(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	})
}

// UserID implements kernel.Authenticator.
func (h *Handler) UserID(ctx context.Context) (int64, bool) {
	u, ok := ctx.Value(userKey{}).(ports.User)
	return u.ID, ok
}

func bearer(r *http.Request) string {
	const prefix = "Bearer "
	if a := r.Header.Get("Authorization"); len(a) > len(prefix) && strings.EqualFold(a[:len(prefix)], prefix) {
		return strings.TrimSpace(a[len(prefix):])
	}
	return ""
}

func toUserDTO(u ports.User) userDTO {
	return userDTO{ID: u.ID, Name: u.Name, Email: u.Email}
}
