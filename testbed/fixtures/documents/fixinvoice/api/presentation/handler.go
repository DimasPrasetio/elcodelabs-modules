// Package presentation is the fixture's HTTP surface.
package presentation

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/fixinvoice/application"
	"__GO_MODULE__/internal/modules/fixinvoice/ports"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/docstate"
	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
	"__GO_MODULE__/internal/shared/validator"
)

const PermManage = "fixinvoice.manage"

type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, authz: authz}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	g := func(f http.HandlerFunc) http.Handler { return h.authz.Require(PermManage, f) }
	mux.Handle("POST /api/v1/fixinvoice/stock", g(h.addStock))
	mux.Handle("POST /api/v1/fixinvoice/invoices", g(h.create))
	mux.Handle("GET /api/v1/fixinvoice/invoices/{id}", g(h.withID(h.svc.Get)))
	mux.Handle("PUT /api/v1/fixinvoice/invoices/{id}", g(h.update))
	mux.Handle("DELETE /api/v1/fixinvoice/invoices/{id}", g(h.remove))
	mux.Handle("POST /api/v1/fixinvoice/invoices/{id}/post", g(h.post))
	mux.Handle("POST /api/v1/fixinvoice/invoices/{id}/cancel", g(h.cancel))
	mux.Handle("POST /api/v1/fixinvoice/invoices/{id}/amend", g(h.withID(h.svc.Amend)))
}

type lineDTO struct {
	SKU           string       `json:"sku"`
	Qty           money.Qty    `json:"qty"`
	Price         money.Amount `json:"price"`
	Amount        money.Amount `json:"amount"`
	DiscountShare money.Amount `json:"discount_share"`
}

type invoiceDTO struct {
	ID            int64        `json:"id"`
	BranchID      int64        `json:"branch_id"`
	CustomerID    int64        `json:"customer_id"`
	CustomerName  string       `json:"customer_name"`
	CustomerTaxID string       `json:"customer_tax_id"`
	Date          string       `json:"date"`
	Number        string       `json:"number"`
	Status        string       `json:"status"`
	Discount      money.Amount `json:"discount"`
	Total         money.Amount `json:"total"`
	AmendedFrom   int64        `json:"amended_from"`
	Revision      int          `json:"revision"`
	CancelReason  string       `json:"cancel_reason"`
	Lines         []lineDTO    `json:"lines"`
}

func toDTO(inv application.Invoice) invoiceDTO {
	d := invoiceDTO{ID: inv.ID, BranchID: inv.BranchID, CustomerID: inv.CustomerID, CustomerName: inv.CustomerName, CustomerTaxID: inv.CustomerTaxID, Date: inv.Date.String(), Number: inv.Number, Status: string(inv.Status), Discount: inv.Discount, Total: inv.Total,
		AmendedFrom: inv.AmendedFrom, Revision: inv.Revision, CancelReason: inv.CancelReason, Lines: []lineDTO{}}
	for _, l := range inv.Lines {
		d.Lines = append(d.Lines, lineDTO(l))
	}
	return d
}

type invoiceRequest struct {
	CustomerID int64        `json:"customer_id"`
	Date       string       `json:"date"`
	Discount   money.Amount `json:"discount"`
	Lines      []struct {
		SKU   string       `json:"sku"`
		Qty   money.Qty    `json:"qty"`
		Price money.Amount `json:"price"`
	} `json:"lines"`
}

func (in invoiceRequest) invoice() (application.Invoice, error) {
	t, err := time.Parse(time.DateOnly, in.Date)
	if err != nil || len(in.Lines) == 0 {
		return application.Invoice{}, apperror.Validation([]validator.FieldError{{Field: "date", Message: "tanggal dan baris wajib"}})
	}
	inv := application.Invoice{CustomerID: in.CustomerID, Date: kernel.DateOf(t, time.UTC), Discount: in.Discount}
	for _, l := range in.Lines {
		inv.Lines = append(inv.Lines, application.Line{SKU: l.SKU, Qty: l.Qty, Price: l.Price})
	}
	return inv, nil
}

// fail maps the fixture's and the kits' errors to responses.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var closed *ports.PeriodClosedError
	var refused *ports.CustomerRefusedError
	switch {
	case errors.As(err, &refused):
		err = apperror.Conflict(refused.Message)
	case errors.Is(err, application.ErrNotFound):
		err = apperror.NotFound("Faktur tidak ditemukan")
	case errors.Is(err, application.ErrInsufficient):
		err = apperror.Conflict("Stok tidak cukup")
	case errors.Is(err, application.ErrInjected):
		err = apperror.Conflict(err.Error())
	case errors.Is(err, application.ErrAmended):
		err = apperror.Conflict("Faktur ini sudah direvisi")
	case errors.As(err, &closed):
		err = apperror.Conflict(closed.Message)
	case errors.Is(err, docstate.ErrTransition), errors.Is(err, docstate.ErrStale):
		err = apperror.Conflict(err.Error())
	case errors.Is(err, docstate.ErrReason):
		err = apperror.Validation([]validator.FieldError{{Field: "reason", Message: err.Error()}})
	case errors.Is(err, money.ErrOverflow), errors.Is(err, money.ErrScale), errors.Is(err, money.ErrSyntax):
		err = apperror.Validation([]validator.FieldError{{Field: "lines", Message: err.Error()}})
	}
	response.Error(w, r, err)
}

func (h *Handler) addStock(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SKU string    `json:"sku"`
		Qty money.Qty `json:"qty"`
	}
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.AddStock(r.Context(), in.SKU, in.Qty); err != nil {
		fail(w, r, err)
		return
	}
	response.Created(w, "Stok ditambah", nil)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in invoiceRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	inv, err := in.invoice()
	if err == nil {
		inv, err = h.svc.Create(r.Context(), inv)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	response.Created(w, "Faktur dibuat", toDTO(inv))
}

func (h *Handler) withID(fn func(ctx context.Context, id int64) (application.Invoice, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := request.PathInt64(r, "id")
		if err != nil {
			response.Error(w, r, err)
			return
		}
		inv, err := fn(r.Context(), id)
		if err != nil {
			fail(w, r, err)
			return
		}
		response.OK(w, "Faktur", toDTO(inv))
	}
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	var in invoiceRequest
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	inv, err := in.invoice()
	if err == nil {
		inv, err = h.svc.Update(r.Context(), id, inv)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Faktur diubah", toDTO(inv))
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err == nil {
		err = h.svc.Delete(r.Context(), id)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Faktur dihapus", nil)
}

func (h *Handler) post(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	hold, _ := strconv.Atoi(r.URL.Query().Get("hold_ms"))
	inv, err := h.svc.Post(r.Context(), id, time.Duration(hold)*time.Millisecond, r.URL.Query().Get("fail_after_number") == "1")
	if err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Faktur diposting", toDTO(inv))
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	inv, err := h.svc.Cancel(r.Context(), id, in.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	response.OK(w, "Faktur dibatalkan", toDTO(inv))
}
