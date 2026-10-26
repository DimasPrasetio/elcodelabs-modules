// Package presentation is the rbac module's HTTP API. Managing roles needs rbac.manage;
// /api/v1/rbac/me only a session.
package presentation

import (
	"net/http"
	"strconv"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/rbac/application"
	"__GO_MODULE__/internal/modules/rbac/domain"
	"__GO_MODULE__/internal/modules/rbac/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
)

// PermManage is the permission this module declares (rbac.module.go) and guards with.
const PermManage = domain.PermManage

type roleDTO struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"is_system"`
	Permissions []string  `json:"permissions"`
	MemberCount int64     `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type userDTO struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	IsActive bool   `json:"is_active"`
	Missing  bool   `json:"missing,omitempty"`
}

type roleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type memberRequest struct {
	UserID   int64 `json:"user_id"`
	BranchID int64 `json:"branch_id"` // 0 or absent: across the whole company
}

type memberDTO struct {
	userDTO
	BranchID int64 `json:"branch_id"` // 0: across the whole company
}

func toRoleDTO(r domain.Role) roleDTO {
	return roleDTO{ID: r.ID, Name: r.Name, Description: r.Description, IsSystem: r.IsSystem, Permissions: r.Permissions, MemberCount: r.MemberCount, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func toUserDTO(u ports.User, missing bool) userDTO {
	return userDTO{ID: u.ID, Name: u.Name, Email: u.Email, IsActive: u.IsActive, Missing: missing}
}

// Handler serves /api/v1/rbac.
type Handler struct {
	svc   *application.Service
	auth  kernel.Authenticator
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, auth kernel.Authenticator, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, auth: auth, authz: authz}
}

// Routes mounts the rbac endpoints.
func (h *Handler) Routes(mux *http.ServeMux) {
	manage := func(f http.HandlerFunc) http.Handler { return h.authz.Require(PermManage, f) }
	mux.Handle("GET /api/v1/rbac/me", h.auth.Require(http.HandlerFunc(h.me)))
	mux.Handle("GET /api/v1/rbac/permissions", manage(h.permissions))
	mux.Handle("GET /api/v1/rbac/roles", manage(h.listRoles))
	mux.Handle("POST /api/v1/rbac/roles", manage(h.createRole))
	mux.Handle("GET /api/v1/rbac/roles/{id}", manage(h.getRole))
	mux.Handle("PUT /api/v1/rbac/roles/{id}", manage(h.updateRole))
	mux.Handle("DELETE /api/v1/rbac/roles/{id}", manage(h.deleteRole))
	mux.Handle("GET /api/v1/rbac/roles/{id}/members", manage(h.members))
	mux.Handle("POST /api/v1/rbac/roles/{id}/members", manage(h.addMember))
	mux.Handle("DELETE /api/v1/rbac/roles/{id}/members/{userId}", manage(h.removeMember))
	mux.Handle("GET /api/v1/rbac/users", manage(h.searchUsers))
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	id, _ := h.auth.UserID(r.Context())
	perms, err := h.svc.Effective(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Izin pengguna", map[string]any{"permissions": perms})
}

func (h *Handler) permissions(w http.ResponseWriter, r *http.Request) {
	response.OK(w, "Daftar izin", h.svc.Catalog())
}

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.svc.ListRoles(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]roleDTO, len(roles))
	for i, role := range roles {
		out[i] = toRoleDTO(role)
	}
	response.OK(w, "Daftar peran", out)
}

func (h *Handler) getRole(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	role, err := h.svc.GetRole(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Data peran", toRoleDTO(role))
}

func (h *Handler) createRole(w http.ResponseWriter, r *http.Request) {
	var in roleRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	actor, _ := h.auth.UserID(r.Context())
	role, err := h.svc.CreateRole(r.Context(), actor, application.RoleInput(in))
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Peran dibuat", toRoleDTO(role))
}

func (h *Handler) updateRole(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	var in roleRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	actor, _ := h.auth.UserID(r.Context())
	role, err := h.svc.UpdateRole(r.Context(), actor, id, application.RoleInput(in))
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Peran diperbarui", toRoleDTO(role))
}

func (h *Handler) deleteRole(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	actor, _ := h.auth.UserID(r.Context())
	if err := h.svc.DeleteRole(r.Context(), actor, id); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Peran dihapus", nil)
}

func (h *Handler) members(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	members, err := h.svc.Members(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]memberDTO, len(members))
	for i, m := range members {
		out[i] = memberDTO{userDTO: toUserDTO(m.User, m.Missing), BranchID: m.BranchID}
	}
	response.OK(w, "Anggota peran", out)
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	var in memberRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	actor, _ := h.auth.UserID(r.Context())
	if err := h.svc.AddMember(r.Context(), actor, id, in.UserID, in.BranchID); err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Peran diberikan", nil)
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	userID, err := request.PathInt64(r, "userId")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	var branch int64
	if b := r.URL.Query().Get("branch"); b != "" {
		if branch, err = strconv.ParseInt(b, 10, 64); err != nil || branch < 0 {
			response.Error(w, r, apperror.BadRequest("Parameter branch tidak valid"))
			return
		}
	}
	actor, _ := h.auth.UserID(r.Context())
	if err := h.svc.RemoveMember(r.Context(), actor, id, userID, branch); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Peran dicabut", nil)
}

func (h *Handler) searchUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.SearchUsers(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]userDTO, len(users))
	for i, u := range users {
		out[i] = toUserDTO(u, false)
	}
	response.OK(w, "Daftar pengguna", out)
}
