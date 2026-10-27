// Package presentation is the numbering module's HTTP API.
package presentation

import (
	"net/http"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/numbering/application"
	"__GO_MODULE__/internal/modules/numbering/domain"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
)

// Handler serves /api/v1/numbering.
type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, authz: authz}
}

// Routes mounts the handlers: reading needs numbering.read; changes need numbering.manage,
// which the service checks across the company (or in the branch of a per-branch series).
func (h *Handler) Routes(mux *http.ServeMux) {
	read := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermRead, f) }
	manage := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermManage, f) }
	mux.Handle("GET /api/v1/numbering/types", read(h.overview))
	mux.Handle("POST /api/v1/numbering/sample", read(h.sample))
	mux.Handle("PUT /api/v1/numbering/types/{docType}", manage(h.setFormat))
	mux.Handle("DELETE /api/v1/numbering/types/{docType}", manage(h.resetFormat))
	mux.Handle("PUT /api/v1/numbering/types/{docType}/next", manage(h.raiseNext))
}

type formatDTO struct {
	Pattern   string `json:"pattern"`
	Reset     string `json:"reset"`
	PerBranch bool   `json:"per_branch"`
}

func toFormat(f domain.Format) formatDTO {
	return formatDTO{Pattern: f.Pattern, Reset: string(f.Reset), PerBranch: f.PerBranch}
}

func (d formatDTO) domain() domain.Format {
	return domain.Format{Pattern: d.Pattern, Reset: domain.Reset(d.Reset), PerBranch: d.PerBranch}
}

type seriesDTO struct {
	BranchID   int64  `json:"branch_id"`
	BranchName string `json:"branch_name"`
	LastValue  int64  `json:"last_value"`
	Next       string `json:"next"`
	Manageable bool   `json:"manageable"`
}

type typeDTO struct {
	DocType   string      `json:"doc_type"`
	Label     string      `json:"label"`
	Format    formatDTO   `json:"format"`
	Default   formatDTO   `json:"default"`
	Custom    bool        `json:"custom"`
	UpdatedAt *time.Time  `json:"updated_at"`
	Series    []seriesDTO `json:"series"` // never null

}

type overviewDTO struct {
	Types []typeDTO `json:"types"`
	Today string    `json:"today"`
	// ManageFormats: the user may change formats (numbering.manage across the company).
	ManageFormats bool `json:"manage_formats"`
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	types, today, manageFormats, err := h.svc.Overview(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := overviewDTO{Types: make([]typeDTO, len(types)), Today: today.String(), ManageFormats: manageFormats}
	for i, t := range types {
		d := typeDTO{DocType: t.Name, Label: t.Label, Format: toFormat(t.Format), Default: toFormat(t.Default), Custom: t.Custom, Series: []seriesDTO{}}
		if t.Custom {
			at := t.UpdatedAt
			d.UpdatedAt = &at
		}
		for _, s := range t.Series {
			d.Series = append(d.Series, seriesDTO{BranchID: s.BranchID, BranchName: s.BranchName, LastValue: s.LastValue, Next: s.Next, Manageable: s.Manageable})
		}
		out.Types[i] = d
	}
	response.OK(w, "Penomoran dokumen", out)
}

func (h *Handler) sample(w http.ResponseWriter, r *http.Request) {
	var in formatDTO
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	s, err := h.svc.Sample(r.Context(), in.domain())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Contoh nomor", map[string]string{"sample": s})
}

func (h *Handler) setFormat(w http.ResponseWriter, r *http.Request) {
	var in formatDTO
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.SetFormat(r.Context(), r.PathValue("docType"), in.domain()); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Format nomor disimpan", nil)
}

func (h *Handler) resetFormat(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.ResetFormat(r.Context(), r.PathValue("docType")); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Format nomor dikembalikan ke bawaan", nil)
}

type nextRequest struct {
	BranchID int64  `json:"branch_id"`
	Next     int64  `json:"next"`
	Reason   string `json:"reason"`
}

func (h *Handler) raiseNext(w http.ResponseWriter, r *http.Request) {
	var in nextRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.RaiseNext(r.Context(), r.PathValue("docType"), in.BranchID, in.Next, in.Reason); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Nomor berikutnya disimpan", nil)
}
