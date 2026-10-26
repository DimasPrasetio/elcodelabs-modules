// Package presentation is the users module's HTTP API. Reading needs users.read, changing
// needs users.manage (kernel Authz: without an authorization module, any signed-in user).
package presentation

import (
	"net/http"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/users/application"
	"__GO_MODULE__/internal/modules/users/domain"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
)

type userDTO struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toDTO(u domain.User) userDTO {
	return userDTO{ID: u.ID, Name: u.Name, Email: u.Email, IsActive: u.IsActive, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt}
}

type createRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	IsActive *bool  `json:"is_active"`
}

type updateRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	// IsActive left out keeps the current state.
	IsActive *bool `json:"is_active"`
}

// Permissions this module declares (users.module.go) and guards with.
const (
	PermRead   = "users.read"
	PermManage = "users.manage"
)

// Handler serves /api/v1/users.
type Handler struct {
	svc   *application.Service
	auth  kernel.Authenticator
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, auth kernel.Authenticator, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, auth: auth, authz: authz}
}

// Routes mounts the handlers, each behind its permission.
func (h *Handler) Routes(mux *http.ServeMux) {
	need := func(perm string, f http.HandlerFunc) http.Handler { return h.authz.Require(perm, f) }
	mux.Handle("GET /api/v1/users", need(PermRead, h.list))
	mux.Handle("GET /api/v1/users/{id}", need(PermRead, h.get))
	mux.Handle("POST /api/v1/users", need(PermManage, h.create))
	mux.Handle("PUT /api/v1/users/{id}", need(PermManage, h.update))
	// The signed-in user's own profile: no permission beyond being signed in.
	mux.Handle("GET /api/v1/account/profile", h.auth.Require(http.HandlerFunc(h.profile)))
	mux.Handle("PUT /api/v1/account/profile", h.auth.Require(http.HandlerFunc(h.updateProfile)))
}

type profileRequest struct {
	Name string `json:"name"`
}

func (h *Handler) profile(w http.ResponseWriter, r *http.Request) {
	id, _ := h.auth.UserID(r.Context())
	u, err := h.svc.Get(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Profil", toDTO(u))
}

func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	var in profileRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	id, _ := h.auth.UserID(r.Context())
	u, err := h.svc.UpdateProfile(r.Context(), id, strings.TrimSpace(in.Name))
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Profil diperbarui", toDTO(u))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := pagination.FromRequest(r)
	users, total, err := h.svc.List(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")), p)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]userDTO, len(users))
	for i, u := range users {
		out[i] = toDTO(u)
	}
	response.Paginated(w, "Data pengguna", out, pagination.NewMeta(p, total))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	u, err := h.svc.Get(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Data pengguna", toDTO(u))
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in createRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	active := in.IsActive == nil || *in.IsActive
	u, err := h.svc.Create(r.Context(), application.CreateInput{Name: strings.TrimSpace(in.Name), Email: in.Email, Password: in.Password, IsActive: active})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Pengguna ditambahkan", toDTO(u))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	var in updateRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	actor, _ := h.auth.UserID(r.Context())
	u, err := h.svc.Update(r.Context(), actor, id, application.UpdateInput{Name: strings.TrimSpace(in.Name), Email: in.Email, Password: in.Password, IsActive: in.IsActive})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Pengguna diperbarui", toDTO(u))
}
