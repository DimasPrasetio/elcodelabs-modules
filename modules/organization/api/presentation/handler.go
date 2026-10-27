// Package presentation is the organization module's HTTP API.
package presentation

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/organization/application"
	"__GO_MODULE__/internal/modules/organization/domain"
	"__GO_MODULE__/internal/modules/organization/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
)

// Handler serves /api/v1/organization.
type Handler struct {
	svc   *application.Service
	auth  kernel.Authenticator
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, auth kernel.Authenticator, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, auth: auth, authz: authz}
}

// Routes mounts the handlers. /me needs only a session (where am I placed); the rest needs
// organization.read or organization.manage in the company the request works in, and the
// companies themselves organization.companies in the home company (checked in the service).
func (h *Handler) Routes(mux *http.ServeMux) {
	signedIn := func(f http.HandlerFunc) http.Handler { return h.auth.Require(f) }
	read := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermRead, f) }
	manage := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermManage, f) }
	companies := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermCompanies, f) }

	mux.Handle("GET /api/v1/organization/me", signedIn(h.me))
	mux.Handle("PUT /api/v1/organization/me/default", signedIn(h.setDefault))

	mux.Handle("GET /api/v1/organization/companies", companies(h.companies))
	mux.Handle("POST /api/v1/organization/companies", companies(h.createCompany))
	mux.Handle("PUT /api/v1/organization/companies/{id}", companies(h.updateCompany))
	mux.Handle("POST /api/v1/organization/companies/{id}/deactivate", companies(h.companyActive(false)))
	mux.Handle("POST /api/v1/organization/companies/{id}/activate", companies(h.companyActive(true)))
	mux.Handle("POST /api/v1/organization/companies/{id}/placements", companies(h.placeInCompany))
	mux.Handle("GET /api/v1/organization/unplaced", companies(h.unplaced))

	mux.Handle("GET /api/v1/organization/branches", read(h.branches))
	mux.Handle("POST /api/v1/organization/branches", manage(h.createBranch))
	mux.Handle("PUT /api/v1/organization/branches/{id}", manage(h.updateBranch))
	mux.Handle("POST /api/v1/organization/branches/{id}/deactivate", manage(h.branchActive(false)))
	mux.Handle("POST /api/v1/organization/branches/{id}/activate", manage(h.branchActive(true)))

	mux.Handle("GET /api/v1/organization/placements", read(h.placements))
	mux.Handle("POST /api/v1/organization/placements", manage(h.place))
	mux.Handle("DELETE /api/v1/organization/placements/{userId}", manage(h.unplace))
	mux.Handle("GET /api/v1/organization/candidates", manage(h.candidates))
}

// --- DTOs ---------------------------------------------------------------------------------

type unitDTO struct {
	CompanyID   int64  `json:"company_id"`
	CompanyCode string `json:"company_code"`
	CompanyName string `json:"company_name"`
	BranchID    int64  `json:"branch_id"`
	BranchCode  string `json:"branch_code"`
	BranchName  string `json:"branch_name"`
	Placed      bool   `json:"placed"`
}

func toUnit(u application.Unit) unitDTO {
	return unitDTO{CompanyID: u.CompanyID, CompanyCode: u.CompanyCode, CompanyName: u.CompanyName, BranchID: u.BranchID, BranchCode: u.BranchCode, BranchName: u.BranchName, Placed: u.Placed}
}

type scopeDTO struct {
	CompanyID int64  `json:"company_id"`
	BranchID  int64  `json:"branch_id"`
	Timezone  string `json:"timezone"`
}

type meDTO struct {
	Units   []unitDTO `json:"units"`
	Default unitDTO   `json:"default"`
	Current scopeDTO  `json:"current"`
}

type companyDTO struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Timezone  string    `json:"timezone"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

func toCompany(c domain.Company) companyDTO {
	return companyDTO{ID: c.ID, Code: c.Code, Name: c.Name, Timezone: c.Timezone, IsActive: c.IsActive, CreatedAt: c.CreatedAt}
}

type companyRequest struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

type branchDTO struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Timezone  string    `json:"timezone"`
	NITKU     string    `json:"nitku"`
	Address   string    `json:"address"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

func toBranch(b domain.Branch) branchDTO {
	return branchDTO{ID: b.ID, Code: b.Code, Name: b.Name, Timezone: b.Timezone, NITKU: b.NITKU, Address: b.Address, IsActive: b.IsActive, CreatedAt: b.CreatedAt}
}

type branchRequest struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
	NITKU    string `json:"nitku"`
	Address  string `json:"address"`
}

type userDTO struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	IsActive bool   `json:"is_active"`
}

func toUser(u ports.User) userDTO {
	return userDTO{ID: u.ID, Name: u.Name, Email: u.Email, IsActive: u.IsActive}
}

type personDTO struct {
	userDTO
	BranchID  int64 `json:"branch_id"`
	IsDefault bool  `json:"is_default"`
	Missing   bool  `json:"missing"`
}

type placeRequest struct {
	UserID   int64 `json:"user_id"`
	BranchID int64 `json:"branch_id"` // 0 or absent: across the company
}

type defaultRequest struct {
	CompanyID int64 `json:"company_id"`
	BranchID  int64 `json:"branch_id"`
}

// --- handlers -----------------------------------------------------------------------------

func (h *Handler) actor(r *http.Request) int64 {
	id, _ := h.auth.UserID(r.Context())
	return id
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	units, def, err := h.svc.Mine(r.Context(), h.actor(r))
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := meDTO{Units: make([]unitDTO, len(units)), Default: toUnit(def)}
	for i, u := range units {
		out.Units[i] = toUnit(u)
	}
	sc := kernel.ScopeFrom(r.Context())
	out.Current = scopeDTO{CompanyID: sc.CompanyID, BranchID: sc.BranchID}
	if sc.Location != nil {
		out.Current.Timezone = sc.Location.String()
	}
	response.OK(w, "Penempatan saya", out)
}

func (h *Handler) setDefault(w http.ResponseWriter, r *http.Request) {
	var in defaultRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.SetDefault(r.Context(), h.actor(r), in.CompanyID, in.BranchID); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Penempatan default disimpan", nil)
}

func (h *Handler) companies(w http.ResponseWriter, r *http.Request) {
	cs, err := h.svc.Companies(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]companyDTO, len(cs))
	for i, c := range cs {
		out[i] = toCompany(c)
	}
	response.OK(w, "Data perusahaan", out)
}

func (h *Handler) createCompany(w http.ResponseWriter, r *http.Request) {
	var in companyRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	c, err := h.svc.CreateCompany(r.Context(), h.actor(r), application.CompanyInput(in))
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Perusahaan dibuat", toCompany(c))
}

func (h *Handler) updateCompany(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	var in companyRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	c, err := h.svc.UpdateCompany(r.Context(), id, application.CompanyInput(in))
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Perusahaan diperbarui", toCompany(c))
}

func (h *Handler) companyActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := request.PathInt64(r, "id")
		if err != nil {
			response.Error(w, r, err)
			return
		}
		if err := h.svc.SetCompanyActive(r.Context(), id, active); err != nil {
			response.Error(w, r, err)
			return
		}
		response.OK(w, map[bool]string{true: "Perusahaan diaktifkan", false: "Perusahaan dinonaktifkan"}[active], nil)
	}
}

type companyPlaceRequest struct {
	Email string `json:"email"`
}

func (h *Handler) placeInCompany(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	var in companyPlaceRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.PlaceInCompany(r.Context(), h.actor(r), id, in.Email); err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Pengguna ditempatkan", nil)
}

func (h *Handler) unplaced(w http.ResponseWriter, r *http.Request) {
	us, err := h.svc.Unplaced(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]userDTO, len(us))
	for i, u := range us {
		out[i] = toUser(u)
	}
	response.OK(w, "Pengguna tanpa penempatan", out)
}

func (h *Handler) branches(w http.ResponseWriter, r *http.Request) {
	bs, err := h.svc.Branches(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]branchDTO, len(bs))
	for i, b := range bs {
		out[i] = toBranch(b)
	}
	response.OK(w, "Data cabang", out)
}

func (h *Handler) createBranch(w http.ResponseWriter, r *http.Request) {
	var in branchRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	b, err := h.svc.CreateBranch(r.Context(), application.BranchInput(in))
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Cabang dibuat", toBranch(b))
}

func (h *Handler) updateBranch(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	var in branchRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	b, err := h.svc.UpdateBranch(r.Context(), id, application.BranchInput(in))
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Cabang diperbarui", toBranch(b))
}

func (h *Handler) branchActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := request.PathInt64(r, "id")
		if err != nil {
			response.Error(w, r, err)
			return
		}
		if err := h.svc.SetBranchActive(r.Context(), id, active); err != nil {
			response.Error(w, r, err)
			return
		}
		response.OK(w, map[bool]string{true: "Cabang diaktifkan", false: "Cabang dinonaktifkan"}[active], nil)
	}
}

func (h *Handler) placements(w http.ResponseWriter, r *http.Request) {
	people, err := h.svc.People(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]personDTO, len(people))
	for i, p := range people {
		out[i] = personDTO{userDTO: toUser(p.User), BranchID: p.BranchID, IsDefault: p.IsDefault, Missing: p.Missing}
	}
	response.OK(w, "Penempatan", out)
}

func (h *Handler) place(w http.ResponseWriter, r *http.Request) {
	var in placeRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	if in.UserID <= 0 {
		response.Error(w, r, apperror.BadRequest("user_id wajib diisi"))
		return
	}
	if err := h.svc.Place(r.Context(), h.actor(r), in.UserID, in.BranchID); err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Pengguna ditempatkan", nil)
}

func (h *Handler) unplace(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.Unplace(r.Context(), h.actor(r), userID, branch); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Penempatan dicabut", nil)
}

func (h *Handler) candidates(w http.ResponseWriter, r *http.Request) {
	us, err := h.svc.Candidates(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")))
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]userDTO, len(us))
	for i, u := range us {
		out[i] = toUser(u)
	}
	response.OK(w, "Pengguna", out)
}
