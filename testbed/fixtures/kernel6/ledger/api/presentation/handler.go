// Package presentation is the ledger's HTTP surface.
package presentation

import (
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/ledger/application"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
)

type Handler struct {
	svc  *application.Service
	auth *kernel.AuthSlot
}

func NewHandler(svc *application.Service, auth *kernel.AuthSlot) *Handler {
	return &Handler{svc: svc, auth: auth}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/ledger/stock", h.auth.Require(http.HandlerFunc(h.add)))
}

func (h *Handler) add(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SKU string `json:"sku"`
		Qty int    `json:"qty"`
	}
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.Add(r.Context(), in.SKU, in.Qty); err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Stok ditambah", nil)
}
