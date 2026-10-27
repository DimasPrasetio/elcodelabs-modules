// Package presentation is the suppliers module's HTTP API.
package presentation

import (
	"errors"
	"net/http"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/suppliers/application"
	"__GO_MODULE__/internal/modules/suppliers/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/pagination"
	"__GO_MODULE__/internal/shared/request"
	"__GO_MODULE__/internal/shared/response"
	"__GO_MODULE__/internal/shared/taxid"
)

// Handler serves /api/v1/suppliers.
type Handler struct {
	svc   *application.Service
	authz *kernel.AuthzSlot
}

func NewHandler(svc *application.Service, authz *kernel.AuthzSlot) *Handler {
	return &Handler{svc: svc, authz: authz}
}

// Routes mounts the handlers. Reading needs suppliers.read, changes suppliers.manage; whole tax
// IDs and bank accounts, and changes to bank accounts, need suppliers.sensitive, which the
// service checks across the company.
func (h *Handler) Routes(mux *http.ServeMux) {
	read := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermRead, f) }
	manage := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermManage, f) }
	const base = "/api/v1/suppliers"
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
	sensitive := func(f http.HandlerFunc) http.Handler { return h.authz.Require(application.PermSensitive, f) }
	mux.Handle("POST "+base+"/{id}/bank-accounts", sensitive(h.saveBankAccount))
	mux.Handle("PUT "+base+"/{id}/bank-accounts/{bid}", sensitive(h.saveBankAccount))
	mux.Handle("POST "+base+"/{id}/bank-accounts/{bid}/deactivate", sensitive(h.setBankAccountActive(false)))
	mux.Handle("POST "+base+"/{id}/bank-accounts/{bid}/activate", sensitive(h.setBankAccountActive(true)))
	mux.Handle("POST "+base+"/{id}/contacts", manage(h.saveContact))
	mux.Handle("PUT "+base+"/{id}/contacts/{cid}", manage(h.saveContact))
	mux.Handle("DELETE "+base+"/{id}/contacts/{cid}", manage(h.deleteContact))
}

type supplierDTO struct {
	ID          int64     `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	LegalName   string    `json:"legal_name"`
	PartyType   string    `json:"party_type"`
	IsForeign   bool      `json:"is_foreign"`
	Country     string    `json:"country"`
	TaxIDKind   string    `json:"tax_id_kind"`
	TaxID       string    `json:"tax_id"`
	TaxIDMasked bool      `json:"tax_id_masked"`
	IsPKP       bool      `json:"is_pkp"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	PaymentDays *int      `json:"payment_days"`
	OnHold      bool      `json:"on_hold"`
	HoldReason  string    `json:"hold_reason"`
	Notes       string    `json:"notes"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toDTO(c domain.Supplier) supplierDTO {
	return supplierDTO{
		ID: c.ID, Code: c.Code, Name: c.Name, LegalName: c.LegalName, PartyType: string(c.PartyType), IsForeign: c.IsForeign,
		Country: c.Country, TaxIDKind: string(c.TaxIDKind), TaxID: c.TaxID,
		TaxIDMasked: c.TaxIDKind == taxid.NIK || c.TaxIDKind == taxid.Passport,
		IsPKP:       c.IsPKP, Email: c.Email, Phone: c.Phone, PaymentDays: c.PaymentDays,
		OnHold: c.OnHold, HoldReason: c.HoldReason, Notes: c.Notes, IsActive: c.IsActive,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func toDTOs(cs []domain.Supplier) []supplierDTO {
	out := make([]supplierDTO, len(cs))
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
	response.Paginated(w, "Daftar pemasok", toDTOs(rows), pagination.NewMeta(p, total))
}

type lookupDTO struct {
	ID     int64  `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	OnHold bool   `json:"on_hold"`
}

// lookup is the active suppliers matching ?q=, for pickers in other modules' pages.
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
	response.OK(w, "Pemasok", out)
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
	Sensitive bool `json:"sensitive"`
	Settings  bool `json:"settings"`
}

type detailDTO struct {
	Supplier     supplierDTO  `json:"supplier"`
	Addresses    []addressDTO `json:"addresses"`
	BankAccounts []bankDTO    `json:"bank_accounts"`
	Contacts     []contactDTO `json:"contacts"`
	Can          canDTO       `json:"can"`
}

func (h *Handler) can(r *http.Request) canDTO {
	ctx := r.Context()
	return canDTO{
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
	out := detailDTO{Supplier: toDTO(d.Supplier), Addresses: make([]addressDTO, len(d.Addresses)), BankAccounts: toBankDTOs(d.BankAccounts),
		Contacts: make([]contactDTO, len(d.Contacts)), Can: h.can(r)}
	for i, a := range d.Addresses {
		out.Addresses[i] = toAddressDTO(a)
	}
	for i, c := range d.Contacts {
		out.Contacts[i] = toContactDTO(c)
	}
	response.OK(w, "Pemasok", out)
}

// supplierInput is a supplier as the form sends it.
type supplierInput struct {
	Code             string `json:"code"`
	Name             string `json:"name"`
	LegalName        string `json:"legal_name"`
	PartyType        string `json:"party_type"`
	IsForeign        bool   `json:"is_foreign"`
	Country          string `json:"country"`
	TaxIDKind        string `json:"tax_id_kind"`
	TaxID            string `json:"tax_id"`
	IsPKP            bool   `json:"is_pkp"`
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	PaymentDays      *int   `json:"payment_days"`
	Notes            string `json:"notes"`
	ConfirmDuplicate bool   `json:"confirm_duplicate"`
}

func (in supplierInput) input() (application.Input, error) {
	c := domain.Supplier{
		Code: in.Code, Name: in.Name, LegalName: in.LegalName, PartyType: domain.PartyType(in.PartyType), IsForeign: in.IsForeign,
		Country: in.Country, TaxIDKind: taxid.Kind(in.TaxIDKind), TaxID: in.TaxID, IsPKP: in.IsPKP,
		Email: in.Email, Phone: in.Phone, PaymentDays: in.PaymentDays, Notes: in.Notes,
	}
	return application.Input{Supplier: c, ConfirmDuplicate: in.ConfirmDuplicate}, nil
}

type savedDTO struct {
	Supplier supplierDTO `json:"supplier"`
	Warnings []string    `json:"warnings"`
}

// fail writes err; a duplicate tax ID is a 409 whose errors list the other suppliers.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var dup *application.DuplicateTaxIDError
	if errors.As(err, &dup) {
		response.Fail(w, http.StatusConflict, "Identitas pajak ini sudah dipakai pemasok lain. Simpan lagi untuk tetap memakainya.", dup.Duplicates)
		return
	}
	response.Error(w, r, err)
}

func (h *Handler) save(w http.ResponseWriter, r *http.Request, id int64) {
	var body supplierInput
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
	out := savedDTO{Supplier: toDTO(saved.Supplier), Warnings: saved.Warnings}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	if id == 0 {
		response.Created(w, "Pemasok ditambahkan", out)
		return
	}
	response.OK(w, "Pemasok disimpan", out)
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
		response.OK(w, map[bool]string{true: "Pemasok diaktifkan", false: "Pemasok dinonaktifkan"}[active], nil)
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
	response.OK(w, "Pemasok ditahan", nil)
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
	response.OK(w, "Tahanan pemasok dilepas", nil)
}

func (h *Handler) reveal(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	c, banks, err := h.svc.Reveal(r.Context(), id)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	response.OK(w, "Identitas pajak dan rekening", revealDTO{TaxIDKind: string(c.TaxIDKind), TaxID: c.TaxID, BankAccounts: toBankDTOs(banks)})
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
	response.OK(w, "Pengaturan pemasok", settingsDTO{CodePrefix: st.CodePrefix, NextCode: domain.GeneratedCode(st.CodePrefix, st.LastNumber+1),
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
	response.OK(w, "Pengaturan pemasok disimpan", nil)
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

type revealDTO struct {
	TaxIDKind    string    `json:"tax_id_kind"`
	TaxID        string    `json:"tax_id"`
	BankAccounts []bankDTO `json:"bank_accounts"`
}

type bankDTO struct {
	ID            int64  `json:"id"`
	BankName      string `json:"bank_name"`
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder"`
	IsDefault     bool   `json:"is_default"`
	IsActive      bool   `json:"is_active"`
}

func toBankDTO(b domain.BankAccount) bankDTO {
	return bankDTO{ID: b.ID, BankName: b.BankName, BankCode: b.BankCode, AccountNumber: b.AccountNumber, AccountHolder: b.AccountHolder,
		IsDefault: b.IsDefault, IsActive: b.IsActive}
}

func toBankDTOs(bs []domain.BankAccount) []bankDTO {
	out := make([]bankDTO, len(bs))
	for i, b := range bs {
		out[i] = toBankDTO(b)
	}
	return out
}

type bankSavedDTO struct {
	BankAccount bankDTO  `json:"bank_account"`
	Warnings    []string `json:"warnings"`
}

func (h *Handler) saveBankAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in bankDTO
	if err := request.DecodeJSON(w, r, &in); err != nil {
		response.Error(w, r, err)
		return
	}
	b := domain.BankAccount{BankName: in.BankName, BankCode: in.BankCode, AccountNumber: in.AccountNumber, AccountHolder: in.AccountHolder, IsDefault: in.IsDefault}
	if r.PathValue("bid") != "" {
		if b.ID, ok = pathID(w, r, "bid"); !ok {
			return
		}
	}
	saved, warnings, err := h.svc.SaveBankAccount(r.Context(), id, b)
	if err != nil {
		response.Error(w, r, err)
		return
	}
	if warnings == nil {
		warnings = []string{}
	}
	response.OK(w, "Rekening disimpan", bankSavedDTO{BankAccount: toBankDTO(saved), Warnings: warnings})
}

func (h *Handler) setBankAccountActive(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(w, r, "id")
		if !ok {
			return
		}
		bid, ok := pathID(w, r, "bid")
		if !ok {
			return
		}
		if err := h.svc.SetBankAccountActive(r.Context(), id, bid, active); err != nil {
			response.Error(w, r, err)
			return
		}
		response.OK(w, map[bool]string{true: "Rekening diaktifkan", false: "Rekening dinonaktifkan"}[active], nil)
	}
}
