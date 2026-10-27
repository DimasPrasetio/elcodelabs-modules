// Package presentation is the customers module's HTTP API.
package presentation

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/customers/application"
	"__GO_MODULE__/internal/modules/customers/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/money"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
	"__GO_MODULE__/internal/shared/taxid"
	"__GO_MODULE__/internal/shared/validator"
)

// Handler serves /api/v1/customers.
type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, authz: authz}
}

// Routes mounts the handlers. Reading needs customers.read, changes customers.manage; terms and
// credit limits (customers.credit) and whole tax IDs (customers.sensitive) are checked across
// the company by the service.
func (h *Handler) Routes(mux *http.ServeMux) {
	read := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermRead, f) }
	manage := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermManage, f) }
	const base = "/api/v1/customers"
	mux.Handle("GET "+base, read(h.list))
	mux.Handle("GET "+base+"/lookup", read(h.lookup))
	mux.Handle("POST "+base, manage(h.create))
	mux.Handle("GET "+base+"/settings", read(h.settings))
	mux.Handle("PUT "+base+"/settings", manage(h.setSettings))
	mux.Handle("GET "+base+"/{id}", read(h.detail))
	mux.Handle("PUT "+base+"/{id}", manage(h.update))
	mux.Handle("POST "+base+"/{id}/deactivate", manage(h.setActive(false)))
	mux.Handle("POST "+base+"/{id}/activate", manage(h.setActive(true)))
	mux.Handle("POST "+base+"/{id}/hold", manage(h.hold))
	mux.Handle("POST "+base+"/{id}/release", manage(h.release))
	mux.Handle("GET "+base+"/{id}/reveal", h.authz.Require(application.PermSensitive, http.HandlerFunc(h.reveal)))
	mux.Handle("POST "+base+"/{id}/addresses", manage(h.saveAddress))
	mux.Handle("PUT "+base+"/{id}/addresses/{aid}", manage(h.saveAddress))
	mux.Handle("POST "+base+"/{id}/addresses/{aid}/deactivate", manage(h.setAddressActive(false)))
	mux.Handle("POST "+base+"/{id}/addresses/{aid}/activate", manage(h.setAddressActive(true)))
	mux.Handle("POST "+base+"/{id}/contacts", manage(h.saveContact))
	mux.Handle("PUT "+base+"/{id}/contacts/{cid}", manage(h.saveContact))
	mux.Handle("DELETE "+base+"/{id}/contacts/{cid}", manage(h.deleteContact))
}

type customerDTO struct {
	ID           int64         `json:"id"`
	Code         string        `json:"code"`
	Name         string        `json:"name"`
	LegalName    string        `json:"legal_name"`
	PartyType    string        `json:"party_type"`
	IsForeign    bool          `json:"is_foreign"`
	Country      string        `json:"country"`
	TaxIDKind    string        `json:"tax_id_kind"`
	TaxID        string        `json:"tax_id"`
	TaxIDMasked  bool          `json:"tax_id_masked"`
	IsPKP        bool          `json:"is_pkp"`
	VATCollector string        `json:"vat_collector"`
	Email        string        `json:"email"`
	Phone        string        `json:"phone"`
	PaymentDays  *int          `json:"payment_days"`
	CreditLimit  *money.Amount `json:"credit_limit"`
	OnHold       bool          `json:"on_hold"`
	HoldReason   string        `json:"hold_reason"`
	Notes        string        `json:"notes"`
	IsActive     bool          `json:"is_active"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

func toDTO(c domain.Customer) customerDTO {
	return customerDTO{
		ID: c.ID, Code: c.Code, Name: c.Name, LegalName: c.LegalName, PartyType: string(c.PartyType), IsForeign: c.IsForeign,
		Country: c.Country, TaxIDKind: string(c.TaxIDKind), TaxID: c.TaxID,
		TaxIDMasked: c.TaxIDKind == taxid.NIK || c.TaxIDKind == taxid.Passport,
		IsPKP:       c.IsPKP, VATCollector: string(c.VATCollector), Email: c.Email, Phone: c.Phone, PaymentDays: c.PaymentDays,
		CreditLimit: c.CreditLimit, OnHold: c.OnHold, HoldReason: c.HoldReason, Notes: c.Notes, IsActive: c.IsActive,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func toDTOs(cs []domain.Customer) []customerDTO {
	out := make([]customerDTO, len(cs))
	for i, c := range cs {
		out[i] = toDTO(c)
	}
	return out
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := request.PathInt64(r, name)
	if err != nil || id <= 0 {
		response.Error(w, r, apperror.NotFound("Tidak ditemukan"))
		return 0, false
	}
	return id, true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := pagination.FromRequest(r)
	q := r.URL.Query()
	rows, total, err := h.svc.List(r.Context(), application.ListFilter{Search: q.Get("q"), Status: q.Get("status"), Limit: p.Limit, Offset: p.Offset()})
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.Paginated(w, "Daftar pelanggan", toDTOs(rows), pagination.NewMeta(p, total))
}

type lookupDTO struct {
	ID     int64  `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	OnHold bool   `json:"on_hold"`
}

// lookup is the active customers matching ?q=, for pickers in other modules' pages.
func (h *Handler) lookup(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.Search(r.Context(), r.URL.Query().Get("q"), 20)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := make([]lookupDTO, len(rows))
	for i, c := range rows {
		out[i] = lookupDTO{ID: c.ID, Code: c.Code, Name: c.Name, OnHold: c.OnHold}
	}
	response.OK(w, "Pelanggan", out)
}

type addressDTO struct {
	ID                int64  `json:"id"`
	Label             string `json:"label"`
	Line              string `json:"line"`
	City              string `json:"city"`
	Province          string `json:"province"`
	PostalCode        string `json:"postal_code"`
	Country           string `json:"country"`
	NITKU             string `json:"nitku"`
	ForBilling        bool   `json:"for_billing"`
	ForShipping       bool   `json:"for_shipping"`
	IsDefaultBilling  bool   `json:"is_default_billing"`
	IsDefaultShipping bool   `json:"is_default_shipping"`
	IsActive          bool   `json:"is_active"`
}

func toAddressDTO(a domain.Address) addressDTO {
	return addressDTO{ID: a.ID, Label: a.Label, Line: a.Line, City: a.City, Province: a.Province, PostalCode: a.PostalCode,
		Country: a.Country, NITKU: a.NITKU, ForBilling: a.ForBilling, ForShipping: a.ForShipping,
		IsDefaultBilling: a.IsDefaultBilling, IsDefaultShipping: a.IsDefaultShipping, IsActive: a.IsActive}
}

func (d addressDTO) domain() domain.Address {
	return domain.Address{Label: d.Label, Line: d.Line, City: d.City, Province: d.Province, PostalCode: d.PostalCode,
		Country: d.Country, NITKU: d.NITKU, ForBilling: d.ForBilling, ForShipping: d.ForShipping,
		IsDefaultBilling: d.IsDefaultBilling, IsDefaultShipping: d.IsDefaultShipping}
}

type contactDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Title     string `json:"title"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	IsPrimary bool   `json:"is_primary"`
}

func toContactDTO(c domain.Contact) contactDTO {
	return contactDTO{ID: c.ID, Name: c.Name, Title: c.Title, Email: c.Email, Phone: c.Phone, IsPrimary: c.IsPrimary}
}

// canDTO tells the web what needs a permission across the company, which useCan cannot answer.
type canDTO struct {
	Credit    bool `json:"credit"`
	Sensitive bool `json:"sensitive"`
	Settings  bool `json:"settings"`
}

type detailDTO struct {
	Customer  customerDTO  `json:"customer"`
	Addresses []addressDTO `json:"addresses"`
	Contacts  []contactDTO `json:"contacts"`
	Can       canDTO       `json:"can"`
}

func (h *Handler) can(r *http.Request) canDTO {
	ctx := r.Context()
	return canDTO{
		Credit:    h.svc.CanCompanyWide(ctx, application.PermCredit),
		Sensitive: h.svc.CanCompanyWide(ctx, application.PermSensitive),
		Settings:  h.svc.CanCompanyWide(ctx, application.PermManage),
	}
}

func (h *Handler) detail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	d, err := h.svc.Detail(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	out := detailDTO{Customer: toDTO(d.Customer), Addresses: make([]addressDTO, len(d.Addresses)), Contacts: make([]contactDTO, len(d.Contacts)), Can: h.can(r)}
	for i, a := range d.Addresses {
		out.Addresses[i] = toAddressDTO(a)
	}
	for i, c := range d.Contacts {
		out.Contacts[i] = toContactDTO(c)
	}
	response.OK(w, "Pelanggan", out)
}

// customerInput is a customer as the form sends it. The credit limit is a string, parsed here so
// a bad one gets a field message.
type customerInput struct {
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	LegalName        string  `json:"legal_name"`
	PartyType        string  `json:"party_type"`
	IsForeign        bool    `json:"is_foreign"`
	Country          string  `json:"country"`
	TaxIDKind        string  `json:"tax_id_kind"`
	TaxID            string  `json:"tax_id"`
	IsPKP            bool    `json:"is_pkp"`
	VATCollector     string  `json:"vat_collector"`
	Email            string  `json:"email"`
	Phone            string  `json:"phone"`
	PaymentDays      *int    `json:"payment_days"`
	CreditLimit      *string `json:"credit_limit"`
	Notes            string  `json:"notes"`
	ConfirmDuplicate bool    `json:"confirm_duplicate"`
}

func (in customerInput) input() (application.Input, error) {
	c := domain.Customer{
		Code: in.Code, Name: in.Name, LegalName: in.LegalName, PartyType: domain.PartyType(in.PartyType), IsForeign: in.IsForeign,
		Country: in.Country, TaxIDKind: taxid.Kind(in.TaxIDKind), TaxID: in.TaxID, IsPKP: in.IsPKP,
		VATCollector: domain.VATCollector(in.VATCollector), Email: in.Email, Phone: in.Phone, PaymentDays: in.PaymentDays, Notes: in.Notes,
	}
	if in.CreditLimit != nil && strings.TrimSpace(*in.CreditLimit) != "" {
		a, err := money.ParseAmount(strings.TrimSpace(*in.CreditLimit))
		if err != nil {
			return application.Input{}, apperror.Validation([]validator.FieldError{{Field: "credit_limit", Message: "angka dengan paling banyak 2 desimal, mis. 50000000"}})
		}
		c.CreditLimit = &a
	}
	return application.Input{Customer: c, ConfirmDuplicate: in.ConfirmDuplicate}, nil
}

type savedDTO struct {
	Customer customerDTO `json:"customer"`
	Warnings []string    `json:"warnings"`
}

// fail writes err; a duplicate tax ID is a 409 whose errors list the other customers.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var dup *application.DuplicateTaxIDError
	if errors.As(err, &dup) {
		response.Fail(w, http.StatusConflict, "Identitas pajak ini sudah dipakai pelanggan lain. Simpan lagi untuk tetap memakainya.", dup.Duplicates)
		return
	}
	response.Error(w, r, err)
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request, id int64) {
	var body customerInput
	if err := request.DecodeJSON(w, r, &body); err != nil {
		response.Error(w, r, err)
		return
	}
	in, err := body.input()
	if err != nil {
		response.Error(w, r, err)
		return
	}
	var saved application.Saved
	if id == 0 {
		saved, err = h.svc.Create(r.Context(), in)
	} else {
		saved, err = h.svc.Update(r.Context(), id, in)
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	out := savedDTO{Customer: toDTO(saved.Customer), Warnings: saved.Warnings}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	if id == 0 {
		response.Created(w, "Pelanggan ditambahkan", out)
		return
	}
	response.OK(w, "Pelanggan disimpan", out)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) { h.save(w, r, 0) }

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	if id, ok := pathID(w, r, "id"); ok {
		h.save(w, r, id)
	}
}

func (h *Handler) setActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "id")
		if !ok {
			return
		}
		if err := h.svc.SetActive(r.Context(), id, active); err != nil {
			response.Error(w, r, err)
			return
		}
		response.OK(w, map[bool]string{true: "Pelanggan diaktifkan", false: "Pelanggan dinonaktifkan"}[active], nil)
	}
}

type reasonInput struct {
	Reason string `json:"reason"`
}

func (h *Handler) hold(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in reasonInput
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.Hold(r.Context(), id, in.Reason); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Pelanggan ditahan", nil)
}

func (h *Handler) release(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Release(r.Context(), id); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Tahanan pelanggan dilepas", nil)
}

func (h *Handler) reveal(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, err := h.svc.Reveal(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Identitas pajak", map[string]string{"tax_id_kind": string(c.TaxIDKind), "tax_id": c.TaxID})
}

type settingsDTO struct {
	CodePrefix string `json:"code_prefix"`
	NextCode   string `json:"next_code"`
	CanManage  bool   `json:"can_manage"`
}

func (h *Handler) settings(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Settings(r.Context())
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Pengaturan pelanggan", settingsDTO{CodePrefix: st.CodePrefix, NextCode: domain.GeneratedCode(st.CodePrefix, st.LastNumber+1),
		CanManage: h.svc.CanCompanyWide(r.Context(), application.PermManage)})
}

func (h *Handler) setSettings(w http.ResponseWriter, r *http.Request) {
	var in settingsDTO
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	if err := h.svc.SetPrefix(r.Context(), in.CodePrefix); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Pengaturan pelanggan disimpan", nil)
}

func (h *Handler) saveAddress(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in addressDTO
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	a := in.domain()
	if r.PathValue("aid") != "" {
		if a.ID, ok = pathID(w, r, "aid"); !ok {
			return
		}
	}
	saved, err := h.svc.SaveAddress(r.Context(), id, a)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Alamat disimpan", toAddressDTO(saved))
}

func (h *Handler) setAddressActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "id")
		if !ok {
			return
		}
		aid, ok := pathID(w, r, "aid")
		if !ok {
			return
		}
		if err := h.svc.SetAddressActive(r.Context(), id, aid, active); err != nil {
			response.Error(w, r, err)
			return
		}
		response.OK(w, map[bool]string{true: "Alamat diaktifkan", false: "Alamat dinonaktifkan"}[active], nil)
	}
}

func (h *Handler) saveContact(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in contactDTO
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	c := domain.Contact{Name: in.Name, Title: in.Title, Email: in.Email, Phone: in.Phone, IsPrimary: in.IsPrimary}
	if r.PathValue("cid") != "" {
		if c.ID, ok = pathID(w, r, "cid"); !ok {
			return
		}
	}
	saved, err := h.svc.SaveContact(r.Context(), id, c)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Kontak disimpan", toContactDTO(saved))
}

func (h *Handler) deleteContact(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	cid, ok := pathID(w, r, "cid")
	if !ok {
		return
	}
	if err := h.svc.DeleteContact(r.Context(), id, cid); err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Kontak dihapus", nil)
}
