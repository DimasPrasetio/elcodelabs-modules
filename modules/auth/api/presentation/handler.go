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

// Cookie names. HttpOnly: page scripts never see the token. In production the __Host-
// prefix makes the browser refuse the cookie unless it is Secure, host-only, and Path=/ — so a
// sibling subdomain cannot plant or overwrite it. Plain-HTTP development keeps a plain name.
const (
	CookieName    = "__Host-session"
	DevCookieName = "session"
)

type userDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type sessionDTO struct {
	User      userDTO   `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
	// Token is only for non-browser clients that ask for it (return_token) and send it as
	// Authorization: Bearer. Browsers use the cookie and never see the token.
	Token string `json:"token,omitempty"`
}

type loginRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	ReturnToken bool   `json:"return_token"`
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

func (h *Handler) cookieName() string {
	if h.secureCookie {
		return CookieName
	}
	return DevCookieName
}

func (h *Handler) setCookie(w http.ResponseWriter, value string, expires time.Time, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: h.cookieName(), Value: value, Path: "/", Expires: expires, MaxAge: maxAge,
		HttpOnly: true, Secure: h.secureCookie,
		// Lax keeps the cookie off cross-site requests; the kernel refuses cross-site writes from
		// sibling subdomains (same site) and accepts JSON bodies only.
		SameSite: http.SameSiteLaxMode,
	})
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
	s, err := h.svc.Login(r.Context(), in.Email, in.Password, application.Client{IP: request.ClientIP(r), UserAgent: r.UserAgent()})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	h.setCookie(w, s.Token, s.ExpiresAt, 0)
	out := sessionDTO{User: toUserDTO(s.User), ExpiresAt: s.ExpiresAt}
	if in.ReturnToken {
		out.Token = s.Token
	}
	response.OK(w, "Berhasil masuk", out)
}

// logout ends the session behind the request's token — the token stops working everywhere,
// not just in this browser — and clears the cookie.
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if token := h.token(r); token != "" {
		if err := h.svc.Logout(r.Context(), token); err != nil {
			response.Error(w, r, err)
			return
		}
	}
	h.setCookie(w, "", time.Time{}, -1)
	response.OK(w, "Berhasil keluar", nil)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	u, _ := r.Context().Value(userKey{}).(ports.User)
	response.OK(w, "Pengguna yang masuk", toUserDTO(u))
}

type userKey struct{}

// Require implements kernel.Authenticator: the token comes from the session cookie or an
// Authorization: Bearer header, and must belong to an active user.
func (h *Handler) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := h.token(r)
		if token == "" {
			response.Error(w, r, apperror.Unauthorized("Silakan masuk terlebih dahulu"))
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

// token is the request's session token: Authorization: Bearer, else the session cookie.
func (h *Handler) token(r *http.Request) string {
	if t := bearer(r); t != "" {
		return t
	}
	if c, err := r.Cookie(h.cookieName()); err == nil {
		return c.Value
	}
	return ""
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
