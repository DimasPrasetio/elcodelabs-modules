// Package application holds the suppliers module's use cases: keeping suppliers, their
// addresses, bank accounts, and contacts, and answering other modules what a document needs to know.
package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/suppliers/domain"
	"__GO_MODULE__/internal/shared/apperror"
	"__GO_MODULE__/internal/shared/taxid"
	"__GO_MODULE__/internal/shared/validator"
)

// Permissions this module declares (suppliers.module.go) and checks.
const (
	PermRead      = "suppliers.read"
	PermManage    = "suppliers.manage"
	PermSensitive = "suppliers.sensitive"
)

var (
	// ErrNotFound is no row with that ID in the company.
	ErrNotFound = errors.New("suppliers: not found")
	// ErrCodeTaken is a code another supplier of the company has.
	ErrCodeTaken = errors.New("suppliers: code taken")
)

// ListFilter is a page of the supplier list.
type ListFilter struct {
	Search string
	Status string // "", "active", "on_hold", "inactive"
	Limit  int
	Offset int
}

// Settings are a company's code prefix and counter.
type Settings struct {
	CodePrefix string
	LastNumber int64
}

// DefaultPrefix is the prefix of a company that never chose one.
const DefaultPrefix = "PMS-"

// Duplicate is another supplier with the same tax ID.
type Duplicate struct {
	ID       int64  `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// DuplicateTaxIDError is a save refused because other suppliers have the tax ID; sending
// ConfirmDuplicate saves it anyway (one NPWP may have several places billed apart).
type DuplicateTaxIDError struct {
	Duplicates []Duplicate
}

func (e *DuplicateTaxIDError) Error() string {
	return fmt.Sprintf("suppliers: tax ID used by %d other suppliers", len(e.Duplicates))
}

// Repository is the supp_* tables.
type Repository interface {
	List(ctx context.Context, company int64, f ListFilter) ([]domain.Supplier, int64, error)
	Search(ctx context.Context, company int64, q string, limit int) ([]domain.Supplier, error)
	Get(ctx context.Context, company, id int64) (domain.Supplier, error)
	// GetShared is Get holding the row shared until the transaction ends.
	GetShared(ctx context.Context, company, id int64) (domain.Supplier, error)
	// GetForUpdate is Get holding the row exclusively until the transaction ends.
	GetForUpdate(ctx context.Context, company, id int64) (domain.Supplier, error)
	WithTaxID(ctx context.Context, company int64, taxID string, except int64) ([]Duplicate, error)
	CodeTaken(ctx context.Context, company int64, code string) (bool, error)
	Insert(ctx context.Context, c domain.Supplier) (int64, error)
	Update(ctx context.Context, c domain.Supplier) error
	SetActive(ctx context.Context, company, id int64, active bool, by int64, at time.Time) error
	SetHold(ctx context.Context, company, id int64, hold bool, reason string, by int64, at time.Time) error

	Addresses(ctx context.Context, company, supplier int64) ([]domain.Address, error)
	Address(ctx context.Context, company, supplier, id int64) (domain.Address, error)
	InsertAddress(ctx context.Context, a domain.Address) (int64, error)
	UpdateAddress(ctx context.Context, a domain.Address) error
	SetAddressActive(ctx context.Context, company, supplier, id int64, active bool, at time.Time) error
	// ClearDefaults takes the default billing and/or shipping mark from every address of the
	// supplier but except.
	ClearDefaults(ctx context.Context, company, supplier, except int64, billing, shipping bool) error

	BankAccounts(ctx context.Context, company, supplier int64) ([]domain.BankAccount, error)
	BankAccount(ctx context.Context, company, supplier, id int64) (domain.BankAccount, error)
	InsertBankAccount(ctx context.Context, b domain.BankAccount) (int64, error)
	UpdateBankAccount(ctx context.Context, b domain.BankAccount) error
	SetBankAccountActive(ctx context.Context, company, supplier, id int64, active bool, at time.Time) error
	ClearDefaultBankAccount(ctx context.Context, company, supplier, except int64) error

	Contacts(ctx context.Context, company, supplier int64) ([]domain.Contact, error)
	Contact(ctx context.Context, company, supplier, id int64) (domain.Contact, error)
	InsertContact(ctx context.Context, c domain.Contact) (int64, error)
	UpdateContact(ctx context.Context, c domain.Contact) error
	DeleteContact(ctx context.Context, company, supplier, id int64) error
	ClearPrimaryContact(ctx context.Context, company, supplier, except int64) error

	// Settings are the company's, or the defaults when it has none yet.
	Settings(ctx context.Context, company int64) (Settings, error)
	// LockSettings creates the company's row when missing and holds it until the transaction ends.
	LockSettings(ctx context.Context, company int64, at time.Time) (Settings, error)
	SetLastNumber(ctx context.Context, company, n int64) error
	SetPrefix(ctx context.Context, company int64, prefix string, at time.Time) error
}

// Transactor runs fn in one transaction — the kernel's d.Tx.
type Transactor interface {
	Do(ctx context.Context, fn func(ctx context.Context) error, opts ...kernel.TxOption) error
}

// Auditor records changes — the kernel's d.Audit.
type Auditor interface {
	Record(ctx context.Context, e kernel.AuditEntry)
}

// Authorizer answers how far the signed-in user's permissions reach — the kernel's d.Authz.
type Authorizer interface {
	Units(ctx context.Context, permission string) (kernel.Units, error)
}

// Identity is who is signed in — the kernel's d.Auth.
type Identity interface {
	UserID(ctx context.Context) (int64, bool)
}

// Service is the suppliers module's use cases.
type Service struct {
	repo  Repository
	tx    Transactor
	audit Auditor
	authz Authorizer
	auth  Identity
	clock kernel.Clock
}

func NewService(repo Repository, tx Transactor, audit Auditor, authz Authorizer, auth Identity, clock kernel.Clock) *Service {
	return &Service{repo: repo, tx: tx, audit: audit, authz: authz, auth: auth, clock: clock}
}

func company(ctx context.Context) int64 { return kernel.ScopeFrom(ctx).CompanyID }

func target(id int64) string { return "supplier:" + strconv.FormatInt(id, 10) }

// companyWide refuses unless the signed-in user holds permission across the whole company.
func (s *Service) companyWide(ctx context.Context, permission, deny string) error {
	u, err := s.authz.Units(ctx, permission)
	if err != nil {
		return err
	}
	if !u.AllBranches {
		return apperror.Forbidden(deny)
	}
	return nil
}

// CanCompanyWide reports whether the signed-in user holds permission across the whole company —
// for the web to show what the API will allow.
func (s *Service) CanCompanyWide(ctx context.Context, permission string) bool {
	u, err := s.authz.Units(ctx, permission)
	return err == nil && u.AllBranches
}

func notFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return apperror.NotFound("Pemasok tidak ditemukan")
	}
	return err
}

// Masked is a supplier as lists and details show it: a NIK or passport number is masked (UU PDP);
// an NPWP of a company is not personal data and shows whole.
func Masked(c domain.Supplier) domain.Supplier {
	if c.TaxIDKind == taxid.NIK || c.TaxIDKind == taxid.Passport {
		c.TaxID = taxid.Mask(c.TaxID)
	}
	return c
}

// List is a page of the company's suppliers, masked.
func (s *Service) List(ctx context.Context, f ListFilter) ([]domain.Supplier, int64, error) {
	f.Search = strings.TrimSpace(f.Search)
	switch f.Status {
	case "", "active", "on_hold", "inactive":
	default:
		return nil, 0, apperror.Validation([]validator.FieldError{{Field: "status", Message: "pilih active, on_hold, atau inactive"}})
	}
	rows, total, err := s.repo.List(ctx, company(ctx), f)
	for i := range rows {
		rows[i] = Masked(rows[i])
	}
	return rows, total, err
}

// Detail is a supplier with its addresses, bank accounts (numbers masked), and contacts.
type Detail struct {
	Supplier     domain.Supplier
	Addresses    []domain.Address
	BankAccounts []domain.BankAccount
	Contacts     []domain.Contact
}

// Detail returns a supplier, active or not, masked, with its addresses, bank accounts, and contacts.
func (s *Service) Detail(ctx context.Context, id int64) (Detail, error) {
	co := company(ctx)
	c, err := s.repo.Get(ctx, co, id)
	if err != nil {
		return Detail{}, notFound(err)
	}
	addrs, err := s.repo.Addresses(ctx, co, id)
	if err != nil {
		return Detail{}, err
	}
	banks, err := s.repo.BankAccounts(ctx, co, id)
	if err != nil {
		return Detail{}, err
	}
	for i := range banks {
		banks[i].AccountNumber = taxid.Mask(banks[i].AccountNumber)
	}
	contacts, err := s.repo.Contacts(ctx, co, id)
	if err != nil {
		return Detail{}, err
	}
	return Detail{Supplier: Masked(c), Addresses: addrs, BankAccounts: banks, Contacts: contacts}, nil
}

// Input is what a person sends to create or change a supplier. Code "" on create takes the
// next generated code. A TaxID equal to the masked stored one keeps the stored one.
type Input struct {
	Supplier         domain.Supplier
	ConfirmDuplicate bool
}

// Saved is a saved supplier, masked, and what to warn about.
type Saved struct {
	Supplier domain.Supplier
	Warnings []string
}

func intPtrEqual(a, b *int) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func termsData(c domain.Supplier) map[string]any {
	d := map[string]any{"payment_days": nil}
	if c.PaymentDays != nil {
		d["payment_days"] = *c.PaymentDays
	}
	return d
}

// Create adds a supplier to the company.
func (s *Service) Create(ctx context.Context, in Input) (Saved, error) {
	c := in.Supplier
	c.Code = domain.NormalizeCode(c.Code)
	var v validator.Validator
	warnings := c.Clean(&v)
	if c.Code != "" {
		v.Check(domain.ValidCode(c.Code), "code", "huruf besar, angka, . / -; maksimal 30")
	}
	if err := v.Err(); err != nil {
		return Saved{}, err
	}
	actor, _ := s.auth.UserID(ctx)
	now := s.clock.Now()
	co := company(ctx)
	c.CompanyID, c.IsActive, c.CreatedBy, c.CreatedAt, c.UpdatedBy, c.UpdatedAt = co, true, actor, now, actor, now
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		if err := s.checkDuplicate(ctx, co, c.TaxID, 0, in.ConfirmDuplicate); err != nil {
			return err
		}
		if c.Code == "" {
			code, err := s.nextCode(ctx, co, now)
			if err != nil {
				return err
			}
			c.Code = code
		}
		id, err := s.repo.Insert(ctx, c)
		if err != nil {
			return codeTaken(err)
		}
		c.ID = id
		s.audit.Record(ctx, kernel.AuditEntry{Module: "suppliers", Action: "supplier.created", Target: target(id),
			Summary: "Menambah pemasok " + c.Code + " " + c.Name, Data: map[string]any{"code": c.Code, "name": c.Name, "terms": termsData(c)}})
		return nil
	})
	if err != nil {
		return Saved{}, err
	}
	return Saved{Supplier: Masked(c), Warnings: warnings}, nil
}

func codeTaken(err error) error {
	if errors.Is(err, ErrCodeTaken) {
		return apperror.Validation([]validator.FieldError{{Field: "code", Message: "sudah dipakai pemasok lain"}})
	}
	return err
}

func (s *Service) checkDuplicate(ctx context.Context, co int64, taxID string, except int64, confirmed bool) error {
	if taxID == "" || confirmed {
		return nil
	}
	dups, err := s.repo.WithTaxID(ctx, co, taxID, except)
	if err != nil {
		return err
	}
	if len(dups) > 0 {
		return &DuplicateTaxIDError{Duplicates: dups}
	}
	return nil
}

// maxCodeSkips bounds how many typed codes one generated code passes over.
const maxCodeSkips = 1000

// nextCode takes the company's next generated code under its settings row lock, passing over
// codes someone typed by hand.
func (s *Service) nextCode(ctx context.Context, co int64, now time.Time) (string, error) {
	st, err := s.repo.LockSettings(ctx, co, now)
	if err != nil {
		return "", err
	}
	for n := st.LastNumber + 1; n <= st.LastNumber+maxCodeSkips; n++ {
		code := domain.GeneratedCode(st.CodePrefix, n)
		taken, err := s.repo.CodeTaken(ctx, co, code)
		if err != nil {
			return "", err
		}
		if !taken {
			return code, s.repo.SetLastNumber(ctx, co, n)
		}
	}
	return "", apperror.Conflict("Kode otomatis berikutnya sudah dipakai semua — ganti awalan kode di pengaturan pemasok")
}

// Update changes a supplier. A change of payment terms is recorded with before and after.
func (s *Service) Update(ctx context.Context, id int64, in Input) (Saved, error) {
	co := company(ctx)
	actor, _ := s.auth.UserID(ctx)
	var saved Saved
	err := s.tx.Do(ctx, func(ctx context.Context) error {
		before, err := s.repo.GetForUpdate(ctx, co, id)
		if err != nil {
			return notFound(err)
		}
		c := in.Supplier
		c.Code = domain.NormalizeCode(c.Code)
		if c.TaxIDKind == before.TaxIDKind && strings.TrimSpace(c.TaxID) == Masked(before).TaxID {
			c.TaxID = before.TaxID // the form sent back what it was shown
		}
		var v validator.Validator
		warnings := c.Clean(&v)
		v.Check(domain.ValidCode(c.Code), "code", "wajib diisi: huruf besar, angka, . / -; maksimal 30")
		if err := v.Err(); err != nil {
			return err
		}
		termsChanged := !intPtrEqual(before.PaymentDays, c.PaymentDays)
		if c.TaxID != before.TaxID {
			if err := s.checkDuplicate(ctx, co, c.TaxID, id, in.ConfirmDuplicate); err != nil {
				return err
			}
		}
		c.ID, c.CompanyID, c.IsActive, c.OnHold, c.HoldReason = id, co, before.IsActive, before.OnHold, before.HoldReason
		c.CreatedBy, c.CreatedAt, c.UpdatedBy, c.UpdatedAt = before.CreatedBy, before.CreatedAt, actor, s.clock.Now()
		if err := s.repo.Update(ctx, c); err != nil {
			return codeTaken(err)
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "suppliers", Action: "supplier.updated", Target: target(id),
			Summary: "Mengubah pemasok " + c.Code + " " + c.Name, Data: map[string]any{"code": c.Code, "name": c.Name}})
		if termsChanged {
			s.audit.Record(ctx, kernel.AuditEntry{Module: "suppliers", Action: "supplier.terms_changed", Target: target(id),
				Summary: "Mengubah termin pembayaran pemasok " + c.Code, Data: map[string]any{"before": termsData(before), "after": termsData(c)}})
		}
		saved = Saved{Supplier: Masked(c), Warnings: warnings}
		return nil
	})
	return saved, err
}

// SetActive deactivates or reactivates a supplier. Its documents stay; new ones may not use it.
func (s *Service) SetActive(ctx context.Context, id int64, active bool) error {
	co := company(ctx)
	actor, _ := s.auth.UserID(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		c, err := s.repo.GetForUpdate(ctx, co, id)
		if err != nil {
			return notFound(err)
		}
		if c.IsActive == active {
			return nil
		}
		if err := s.repo.SetActive(ctx, co, id, active, actor, s.clock.Now()); err != nil {
			return err
		}
		action, summary := "supplier.deactivated", "Menonaktifkan pemasok "
		if active {
			action, summary = "supplier.activated", "Mengaktifkan pemasok "
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "suppliers", Action: action, Target: target(id), Summary: summary + c.Code + " " + c.Name})
		return nil
	})
}

// Hold puts a supplier on hold with a reason: documents check it (Usable) and refuse.
func (s *Service) Hold(ctx context.Context, id int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if !domain.ValidReason(reason) {
		return apperror.Validation([]validator.FieldError{{Field: "reason", Message: "Alasan wajib diisi, 3 sampai 255 karakter"}})
	}
	return s.setHold(ctx, id, true, reason)
}

// Release takes a supplier off hold.
func (s *Service) Release(ctx context.Context, id int64) error { return s.setHold(ctx, id, false, "") }

func (s *Service) setHold(ctx context.Context, id int64, hold bool, reason string) error {
	co := company(ctx)
	actor, _ := s.auth.UserID(ctx)
	return s.tx.Do(ctx, func(ctx context.Context) error {
		c, err := s.repo.GetForUpdate(ctx, co, id)
		if err != nil {
			return notFound(err)
		}
		if c.OnHold == hold && c.HoldReason == reason {
			return nil
		}
		if err := s.repo.SetHold(ctx, co, id, hold, reason, actor, s.clock.Now()); err != nil {
			return err
		}
		action, summary := "supplier.held", "Menahan pemasok "+c.Code+": "+reason
		if !hold {
			action, summary = "supplier.released", "Melepas tahanan pemasok "+c.Code
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "suppliers", Action: action, Target: target(id), Summary: summary, Data: map[string]any{"reason": reason}})
		return nil
	})
}

// Reveal returns a supplier's whole tax ID and bank account numbers: suppliers.sensitive across
// the company, and an access record (UU PDP).
func (s *Service) Reveal(ctx context.Context, id int64) (domain.Supplier, []domain.BankAccount, error) {
	if err := s.companyWide(ctx, PermSensitive, "Melihat identitas pajak dan rekening utuh butuh izin suppliers.sensitive di seluruh perusahaan"); err != nil {
		return domain.Supplier{}, nil, err
	}
	co := company(ctx)
	c, err := s.repo.Get(ctx, co, id)
	if err != nil {
		return domain.Supplier{}, nil, notFound(err)
	}
	banks, err := s.repo.BankAccounts(ctx, co, id)
	if err != nil {
		return domain.Supplier{}, nil, err
	}
	s.audit.Record(ctx, kernel.AuditEntry{Kind: kernel.AuditAccess, Module: "suppliers", Action: "supplier.sensitive_viewed", Target: target(id),
		Summary: "Melihat identitas pajak dan rekening utuh pemasok " + c.Code, Data: map[string]any{"tax_id_kind": string(c.TaxIDKind), "bank_accounts": len(banks)}})
	return c, banks, nil
}

// Settings are the company's code prefix and last generated number.
func (s *Service) Settings(ctx context.Context) (Settings, error) {
	return s.repo.Settings(ctx, company(ctx))
}

// SetPrefix changes the prefix of generated codes: suppliers.manage across the company. Codes
// already given keep theirs.
func (s *Service) SetPrefix(ctx context.Context, prefix string) error {
	prefix = domain.NormalizeCode(prefix)
	if !domain.ValidPrefix(prefix) {
		return apperror.Validation([]validator.FieldError{{Field: "code_prefix", Message: "huruf besar, angka, . / -; maksimal 10"}})
	}
	if err := s.companyWide(ctx, PermManage, "Mengubah awalan kode butuh izin suppliers.manage di seluruh perusahaan"); err != nil {
		return err
	}
	co := company(ctx)
	now := s.clock.Now()
	return s.tx.Do(ctx, func(ctx context.Context) error {
		st, err := s.repo.LockSettings(ctx, co, now)
		if err != nil || st.CodePrefix == prefix {
			return err
		}
		if err := s.repo.SetPrefix(ctx, co, prefix, now); err != nil {
			return err
		}
		s.audit.Record(ctx, kernel.AuditEntry{Module: "suppliers", Action: "settings.changed", Target: "suppliers:settings",
			Summary: "Mengubah awalan kode pemasok menjadi " + prefix, Data: map[string]any{"before": st.CodePrefix, "after": prefix}})
		return nil
	})
}
