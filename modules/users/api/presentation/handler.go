// Package presentation is the users module's HTTP API. Every route requires a signed-in user.
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
	IsActive bool   `json:"is_active"`
}

// Handler serves /api/v1/users.
type Handler struct {
	svc  *application.Service
	auth kernel.Authenticator
}

func NewHandler(svc *application.Service, auth kernel.Authenticator) *Handler {
	return &Handler{svc: svc, auth: auth}
}

// Routes mounts the handlers, each behind the auth guard.
func (h *Handler) Routes(mux *http.ServeMux) {
	guard := func(f http.HandlerFunc) http.Handler { return h.auth.Require(f) }
	mux.Handle("GET /api/v1/users", guard(h.list))
	mux.Handle("POST /api/v1/users", guard(h.create))
	mux.Handle("GET /api/v1/users/{id}", guard(h.get))
	mux.Handle("PUT /api/v1/users/{id}", guard(h.update))
	mux.Handle("DELETE /api/v1/users/{id}", guard(h.delete))
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
	response.Paginated(w, "Users retrieved", out, pagination.NewMeta(p, total))
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
	response.OK(w, "User retrieved", toDTO(u))
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
	response.Created(w, "User created", toDTO(u))
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
	response.OK(w, "User updated", toDTO(u))
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	actor, _ := h.auth.UserID(r.Context())
	if err := h.svc.Delete(r.Context(), actor, id); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "User deleted", nil)
}
