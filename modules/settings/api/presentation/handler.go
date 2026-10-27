// Package presentation is the settings module's HTTP API.
package presentation

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/settings/application"
	"__GO_MODULE__/internal/modules/settings/contracts"
	"__GO_MODULE__/internal/modules/settings/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
)

// PermManage is the permission this module declares (settings.module.go) and guards with.
const PermManage = "settings.manage"

type companyDTO struct {
	// CompanyID is the company the profile belongs to — the request's scope — so the page can
	// show that company's logo (GET …/logo?company=<id>).
	CompanyID int64  `json:"company_id"`
	Name      string `json:"name"`
	LegalName string `json:"legal_name"`
	Address   string `json:"address"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	TaxID     string `json:"tax_id"`
	Timezone  string `json:"timezone"` // deprecated (see contracts.Company)
	HasLogo   bool   `json:"has_logo"`
	// LogoUpload says whether this install can store a logo (a file store is wired).
	LogoUpload bool `json:"logo_upload"`
}

type companyRequest struct {
	Name      string `json:"name"`
	LegalName string `json:"legal_name"`
	Address   string `json:"address"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	TaxID     string `json:"tax_id"`
	Timezone  string `json:"timezone"`
}

// Handler serves /api/v1/settings.
type Handler struct {
	svc   *application.Service
	auth  kernel.Authenticator
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, auth kernel.Authenticator, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, auth: auth, authz: authz}
}

// Routes mounts the handlers. Reading the profile needs a session; changing it settings.manage;
// the logo itself is public — the sign-in page and emails show it — and names its company
// (?company=<id>, default 1), since a public request has no scope.
func (h *Handler) Routes(mux *http.ServeMux) {
	manage := func(f http.HandlerFunc) http.Handler { return h.authz.Require(PermManage, f) }
	mux.Handle("GET /api/v1/settings/company", h.auth.Require(http.HandlerFunc(h.company)))
	mux.Handle("PUT /api/v1/settings/company", manage(h.updateCompany))
	mux.Handle("PUT /api/v1/settings/company/logo", manage(h.uploadLogo))
	mux.Handle("DELETE /api/v1/settings/company/logo", manage(h.removeLogo))
	mux.HandleFunc("GET /api/v1/settings/company/logo", h.logo)
}

func (h *Handler) dto(ctx context.Context, c domain.Company) companyDTO {
	return companyDTO{CompanyID: kernel.ScopeFrom(ctx).CompanyID, Name: c.Name, LegalName: c.LegalName, Address: c.Address, Phone: c.Phone, Email: c.Email, TaxID: c.TaxID,
		Timezone: c.Timezone, HasLogo: c.LogoFileID != 0, LogoUpload: h.svc.Options().LogoUpload}
}

func (h *Handler) company(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Company(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Profil perusahaan", h.dto(r.Context(), c))
}

func (h *Handler) updateCompany(w http.ResponseWriter, r *http.Request) {
	var in companyRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	actor, _ := h.auth.UserID(r.Context())
	c, err := h.svc.UpdateCompany(r.Context(), actor, domain.Company{Name: in.Name, LegalName: in.LegalName, Address: in.Address,
		Phone: in.Phone, Email: in.Email, TaxID: in.TaxID, Timezone: in.Timezone})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Profil perusahaan disimpan", h.dto(r.Context(), c))
}

// uploadLogo streams the "file" part of a multipart form to the file store — never into memory
// or /tmp, which is RAM in the container.
func (h *Handler) uploadLogo(w http.ResponseWriter, r *http.Request) {
	request.AllowTransfer(w, application.LogoMaxBytes)
	r.Body = http.MaxBytesReader(w, r.Body, application.LogoMaxBytes+64<<10)
	mr, err := r.MultipartReader()
	if err != nil {
		response.Error(w, r, apperror.New(http.StatusUnsupportedMediaType, "Kirim logo sebagai multipart/form-data dengan kolom file"))
		return
	}
	actor, _ := h.auth.UserID(r.Context())
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			response.Error(w, r, apperror.BadRequest("Unggahan tidak terbaca"))
			return
		}
		if part.FormName() != "file" {
			continue
		}
		c, err := h.svc.SetLogo(r.Context(), actor, part.FileName(), part)
		if err != nil {
			response.Error(w, r, err)
			return
		}
		response.OK(w, "Logo disimpan", h.dto(r.Context(), c))
		return
	}
	response.Error(w, r, apperror.BadRequest("Kolom file tidak ada"))
}

func (h *Handler) removeLogo(w http.ResponseWriter, r *http.Request) {
	actor, _ := h.auth.UserID(r.Context())
	c, err := h.svc.RemoveLogo(r.Context(), actor)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Logo dihapus", h.dto(r.Context(), c))
}

func (h *Handler) logo(w http.ResponseWriter, r *http.Request) {
	company := kernel.DefaultCompany
	if c := r.URL.Query().Get("company"); c != "" {
		id, err := strconv.ParseInt(c, 10, 64)
		if err != nil || id < 1 {
			response.Error(w, r, apperror.NotFound("Logo belum diatur"))
			return
		}
		company = id
	}
	// No session, so no scope: the company is the one asked for. With an organization module
	// (several companies) there is no default to fall back on.
	rc, kind, err := h.svc.Logo(kernel.WithScope(r.Context(), kernel.Scope{CompanyID: company}))
	if errors.Is(err, contracts.ErrNoLogo) {
		response.Error(w, r, apperror.NotFound("Logo belum diatur"))
		return
	}
	if err != nil {
		response.Error(w, r, err)
		return
	}
	defer rc.Close()
	request.AllowTransfer(w, application.LogoMaxBytes)
	w.Header().Set("Content-Type", kind)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = io.Copy(w, rc)
}
