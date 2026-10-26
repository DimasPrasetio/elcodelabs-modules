// Package presentation is the orders' HTTP surface.
package presentation

import (
	"errors"
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/orders/application"
	"__GO_MODULE__/internal/modules/orders/ports"
	"__GO_MODULE__/internal/shared/apperror"
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
	mux.Handle("POST /api/v1/orders", h.auth.Require(http.HandlerFunc(h.place)))
	mux.Handle("GET /api/v1/orders/count", h.auth.Require(http.HandlerFunc(h.count)))
}

func (h *Handler) place(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SKU string `json:"sku"`
		Qty int    `json:"qty"`
	}
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	id, err := h.svc.Place(r.Context(), in.SKU, in.Qty)
	if errors.Is(err, ports.ErrInsufficient) {
		response.Error(w, r, apperror.Conflict("Stok tidak cukup"))
		return
	}
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Created(w, "Pesanan dibuat", map[string]int64{"id": id})
}

func (h *Handler) count(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.Count(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Jumlah pesanan", map[string]int64{"count": n})
}
