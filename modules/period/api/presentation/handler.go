// Package presentation is the period module's HTTP API.
package presentation

import (
	"net/http"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/period/application"
	"__GO_MODULE__/internal/modules/period/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
	"__GO_MODULE__/internal/shared/validator"
)

// Handler serves /api/v1/period.
type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, authz: authz}
}

// Routes mounts the handlers. Everything needs period.read; closing, reopening, and the fiscal
// year check period.manage or period.unlock across the company in the service.
func (h *Handler) Routes(mux *http.ServeMux) {
	read := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermRead, f) }
	mux.Handle("GET /api/v1/period/locks", read(h.overview))
	mux.Handle("PUT /api/v1/period/locks/{area}", read(h.setLock))
	mux.Handle("PUT /api/v1/period/fiscal-year", read(h.setFiscalYear))
}

type lockDTO struct {
	LockedThrough string    `json:"locked_through"` // YYYY-MM-DD
	UpdatedBy     int64     `json:"updated_by"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type areaDTO struct {
	Area      string   `json:"area"`
	Label     string   `json:"label"`
	Declared  bool     `json:"declared"`
	Own       *lockDTO `json:"own"`       // null: no lock of its own
	Effective *string  `json:"effective"` // null: open
}

type overviewDTO struct {
	Areas                []areaDTO `json:"areas"`
	FiscalYearStartMonth int       `json:"fiscal_year_start_month"`
	Today                string    `json:"today"`
	CanClose             bool      `json:"can_close"`  // period.manage across the company
	CanReopen            bool      `json:"can_reopen"` // period.unlock across the company
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	o, err := h.svc.Overview(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := overviewDTO{Areas: make([]areaDTO, len(o.Areas)), FiscalYearStartMonth: o.FiscalFrom, Today: o.Today.String(), CanClose: o.CanClose, CanReopen: o.CanReopen}
	for i, a := range o.Areas {
		d := areaDTO{Area: a.Area, Label: a.Label, Declared: a.Declared}
		if a.Own != nil {
			d.Own = &lockDTO{LockedThrough: a.Own.LockedThrough.String(), UpdatedBy: a.Own.UpdatedBy, UpdatedAt: a.Own.UpdatedAt}
		}
		if a.Effective != nil {
			s := a.Effective.String()
			d.Effective = &s
		}
		out.Areas[i] = d
	}
	response.OK(w, "Tutup buku", out)
}

type lockRequest struct {
	LockedThrough *string `json:"locked_through"` // null: remove the lock
	Reason        string  `json:"reason"`
}

func (h *Handler) setLock(w http.ResponseWriter, r *http.Request) {
	var in lockRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	var through *kernel.Date
	if in.LockedThrough != nil && *in.LockedThrough != "" {
		d, err := domain.ParseDate(*in.LockedThrough)
		if err != nil {
			response.Error(w, r, apperror.Validation([]validator.FieldError{{Field: "locked_through", Message: "Tanggal tidak valid (YYYY-MM-DD)"}}))
			return
		}
		through = &d
	}
	l, err := h.svc.SetLock(r.Context(), r.PathValue("area"), through, in.Reason)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	var out *lockDTO
	if l != nil {
		out = &lockDTO{LockedThrough: l.LockedThrough.String(), UpdatedBy: l.UpdatedBy, UpdatedAt: l.UpdatedAt}
	}
	response.OK(w, "Tanggal kunci disimpan", out)
}

type fiscalYearRequest struct {
	FiscalYearStartMonth int `json:"fiscal_year_start_month"`
}

func (h *Handler) setFiscalYear(w http.ResponseWriter, r *http.Request) {
	var in fiscalYearRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.SetFiscalStart(r.Context(), in.FiscalYearStartMonth); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Tahun buku disimpan", nil)
}
