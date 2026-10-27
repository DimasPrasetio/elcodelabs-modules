// Package infrastructure stores suppliers, their addresses, bank accounts, contacts, and code
// settings in MySQL
// through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/suppliers/application"
	"__GO_MODULE__/internal/modules/suppliers/domain"
	"__GO_MODULE__/internal/modules/suppliers/infrastructure/sqlc"
	"__GO_MODULE__/internal/shared/taxid"
)

// Repository implements application.Repository.
type Repository struct {
	tx *kernel.TxSlot
}

func NewRepository(tx *kernel.TxSlot) *Repository { return &Repository{tx: tx} }

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func toSupplier(row sqlc.SuppSupplier) (domain.Supplier, error) {
	c := domain.Supplier{
		ID: row.ID, CompanyID: row.CompanyID, Code: row.Code, Name: row.Name, LegalName: row.LegalName,
		PartyType: domain.PartyType(row.PartyType), IsForeign: row.IsForeign, Country: row.Country,
		TaxIDKind: taxid.Kind(row.TaxIDKind), TaxID: row.TaxID, IsPKP: row.IsPkp,
		Email: row.Email, Phone: row.Phone, OnHold: row.OnHold, HoldReason: row.HoldReason, Notes: row.Notes.String,
		IsActive: row.IsActive, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt,
	}
	if row.PaymentDays.Valid {
		d := int(row.PaymentDays.Int32)
		c.PaymentDays = &d
	}
	return c, nil
}

func toSuppliers(rows []sqlc.SuppSupplier) ([]domain.Supplier, error) {
	out := make([]domain.Supplier, len(rows))
	for i, row := range rows {
		c, err := toSupplier(row)
		if err != nil {
			return nil, err
		}
		out[i] = c
	}
	return out, nil
}

func one(row sqlc.SuppSupplier, err error) (domain.Supplier, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Supplier{}, application.ErrNotFound
	}
	if err != nil {
		return domain.Supplier{}, err
	}
	return toSupplier(row)
}

func paymentDays(p *int) sql.NullInt32 {
	if p == nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(*p), Valid: true}
}

func notes(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// duplicate maps MySQL's duplicate-key error (a code in use) to ErrCodeTaken.
func duplicate(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return application.ErrCodeTaken
	}
	return err
}

func (r *Repository) List(ctx context.Context, company int64, f application.ListFilter) ([]domain.Supplier, int64, error) {
	rows, err := r.q(ctx).ListSuppliers(ctx, sqlc.ListSuppliersParams{CompanyID: company, Status: f.Status, Search: f.Search, Limit: int32(f.Limit), Offset: int32(f.Offset)})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q(ctx).CountSuppliers(ctx, sqlc.CountSuppliersParams{CompanyID: company, Status: f.Status, Search: f.Search})
	if err != nil {
		return nil, 0, err
	}
	out, err := toSuppliers(rows)
	return out, total, err
}

func (r *Repository) Search(ctx context.Context, company int64, q string, limit int) ([]domain.Supplier, error) {
	rows, err := r.q(ctx).SearchActiveSuppliers(ctx, sqlc.SearchActiveSuppliersParams{CompanyID: company, Search: q, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	return toSuppliers(rows)
}

func (r *Repository) Get(ctx context.Context, company, id int64) (domain.Supplier, error) {
	return one(r.q(ctx).GetSupplier(ctx, sqlc.GetSupplierParams{CompanyID: company, ID: id}))
}

func (r *Repository) GetShared(ctx context.Context, company, id int64) (domain.Supplier, error) {
	return one(r.q(ctx).GetSupplierShared(ctx, sqlc.GetSupplierSharedParams{CompanyID: company, ID: id}))
}

func (r *Repository) GetForUpdate(ctx context.Context, company, id int64) (domain.Supplier, error) {
	return one(r.q(ctx).GetSupplierForUpdate(ctx, sqlc.GetSupplierForUpdateParams{CompanyID: company, ID: id}))
}

func (r *Repository) WithTaxID(ctx context.Context, company int64, taxID string, except int64) ([]application.Duplicate, error) {
	rows, err := r.q(ctx).SuppliersWithTaxID(ctx, sqlc.SuppliersWithTaxIDParams{CompanyID: company, TaxID: taxID, ExceptID: except})
	if err != nil {
		return nil, err
	}
	out := make([]application.Duplicate, len(rows))
	for i, row := range rows {
		out[i] = application.Duplicate{ID: row.ID, Code: row.Code, Name: row.Name, IsActive: row.IsActive}
	}
	return out, nil
}

func (r *Repository) CodeTaken(ctx context.Context, company int64, code string) (bool, error) {
	n, err := r.q(ctx).CodeTaken(ctx, sqlc.CodeTakenParams{CompanyID: company, Code: code})
	return n > 0, err
}

func (r *Repository) Insert(ctx context.Context, c domain.Supplier) (int64, error) {
	id, err := r.q(ctx).InsertSupplier(ctx, sqlc.InsertSupplierParams{
		CompanyID: c.CompanyID, Code: c.Code, Name: c.Name, LegalName: c.LegalName, PartyType: string(c.PartyType), IsForeign: c.IsForeign,
		Country: c.Country, TaxIDKind: string(c.TaxIDKind), TaxID: c.TaxID, IsPkp: c.IsPKP,
		Email: c.Email, Phone: c.Phone, PaymentDays: paymentDays(c.PaymentDays), Notes: notes(c.Notes),
		CreatedBy: c.CreatedBy, CreatedAt: c.CreatedAt, UpdatedBy: c.UpdatedBy, UpdatedAt: c.UpdatedAt,
	})
	return id, duplicate(err)
}

func (r *Repository) Update(ctx context.Context, c domain.Supplier) error {
	return duplicate(r.q(ctx).UpdateSupplier(ctx, sqlc.UpdateSupplierParams{
		Code: c.Code, Name: c.Name, LegalName: c.LegalName, PartyType: string(c.PartyType), IsForeign: c.IsForeign,
		Country: c.Country, TaxIDKind: string(c.TaxIDKind), TaxID: c.TaxID, IsPkp: c.IsPKP,
		Email: c.Email, Phone: c.Phone, PaymentDays: paymentDays(c.PaymentDays), Notes: notes(c.Notes),
		UpdatedBy: c.UpdatedBy, UpdatedAt: c.UpdatedAt, CompanyID: c.CompanyID, ID: c.ID,
	}))
}

func (r *Repository) SetActive(ctx context.Context, company, id int64, active bool, by int64, at time.Time) error {
	return r.q(ctx).SetSupplierActive(ctx, sqlc.SetSupplierActiveParams{IsActive: active, UpdatedBy: by, UpdatedAt: at, CompanyID: company, ID: id})
}

func (r *Repository) SetHold(ctx context.Context, company, id int64, hold bool, reason string, by int64, at time.Time) error {
	return r.q(ctx).SetSupplierHold(ctx, sqlc.SetSupplierHoldParams{OnHold: hold, HoldReason: reason, UpdatedBy: by, UpdatedAt: at, CompanyID: company, ID: id})
}

func toAddress(row sqlc.SuppAddress) domain.Address {
	return domain.Address{
		ID: row.ID, CompanyID: row.CompanyID, SupplierID: row.SupplierID, Label: row.Label, Line: row.Line, City: row.City,
		Province: row.Province, PostalCode: row.PostalCode, Country: row.Country, NITKU: row.Nitku.String,
		ForBilling: row.ForBilling, ForShipping: row.ForShipping, IsDefaultBilling: row.IsDefaultBilling, IsDefaultShipping: row.IsDefaultShipping,
		IsActive: row.IsActive, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func nitku(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func (r *Repository) Addresses(ctx context.Context, company, supplier int64) ([]domain.Address, error) {
	rows, err := r.q(ctx).ListAddresses(ctx, sqlc.ListAddressesParams{CompanyID: company, SupplierID: supplier})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Address, len(rows))
	for i, row := range rows {
		out[i] = toAddress(row)
	}
	return out, nil
}

func (r *Repository) Address(ctx context.Context, company, supplier, id int64) (domain.Address, error) {
	row, err := r.q(ctx).GetAddress(ctx, sqlc.GetAddressParams{CompanyID: company, SupplierID: supplier, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Address{}, application.ErrNotFound
	}
	return toAddress(row), err
}

func (r *Repository) InsertAddress(ctx context.Context, a domain.Address) (int64, error) {
	return r.q(ctx).InsertAddress(ctx, sqlc.InsertAddressParams{
		CompanyID: a.CompanyID, SupplierID: a.SupplierID, Label: a.Label, Line: a.Line, City: a.City, Province: a.Province,
		PostalCode: a.PostalCode, Country: a.Country, Nitku: nitku(a.NITKU), ForBilling: a.ForBilling, ForShipping: a.ForShipping,
		IsDefaultBilling: a.IsDefaultBilling, IsDefaultShipping: a.IsDefaultShipping, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	})
}

func (r *Repository) UpdateAddress(ctx context.Context, a domain.Address) error {
	return r.q(ctx).UpdateAddress(ctx, sqlc.UpdateAddressParams{
		Label: a.Label, Line: a.Line, City: a.City, Province: a.Province, PostalCode: a.PostalCode, Country: a.Country,
		Nitku: nitku(a.NITKU), ForBilling: a.ForBilling, ForShipping: a.ForShipping, IsDefaultBilling: a.IsDefaultBilling,
		IsDefaultShipping: a.IsDefaultShipping, UpdatedAt: a.UpdatedAt, CompanyID: a.CompanyID, SupplierID: a.SupplierID, ID: a.ID,
	})
}

func (r *Repository) SetAddressActive(ctx context.Context, company, supplier, id int64, active bool, at time.Time) error {
	return r.q(ctx).SetAddressActive(ctx, sqlc.SetAddressActiveParams{IsActive: active, UpdatedAt: at, CompanyID: company, SupplierID: supplier, ID: id})
}

func (r *Repository) ClearDefaults(ctx context.Context, company, supplier, except int64, billing, shipping bool) error {
	if billing {
		if err := r.q(ctx).ClearDefaultBilling(ctx, sqlc.ClearDefaultBillingParams{CompanyID: company, SupplierID: supplier, ID: except}); err != nil {
			return err
		}
	}
	if shipping {
		return r.q(ctx).ClearDefaultShipping(ctx, sqlc.ClearDefaultShippingParams{CompanyID: company, SupplierID: supplier, ID: except})
	}
	return nil
}

func toBankAccount(row sqlc.SuppBankAccount) domain.BankAccount {
	return domain.BankAccount{ID: row.ID, CompanyID: row.CompanyID, SupplierID: row.SupplierID, BankName: row.BankName, BankCode: row.BankCode,
		AccountNumber: row.AccountNumber, AccountHolder: row.AccountHolder, IsDefault: row.IsDefault, IsActive: row.IsActive,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func (r *Repository) BankAccounts(ctx context.Context, company, supplier int64) ([]domain.BankAccount, error) {
	rows, err := r.q(ctx).ListBankAccounts(ctx, sqlc.ListBankAccountsParams{CompanyID: company, SupplierID: supplier})
	if err != nil {
		return nil, err
	}
	out := make([]domain.BankAccount, len(rows))
	for i, row := range rows {
		out[i] = toBankAccount(row)
	}
	return out, nil
}

func (r *Repository) BankAccount(ctx context.Context, company, supplier, id int64) (domain.BankAccount, error) {
	row, err := r.q(ctx).GetBankAccount(ctx, sqlc.GetBankAccountParams{CompanyID: company, SupplierID: supplier, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.BankAccount{}, application.ErrNotFound
	}
	return toBankAccount(row), err
}

func (r *Repository) InsertBankAccount(ctx context.Context, b domain.BankAccount) (int64, error) {
	return r.q(ctx).InsertBankAccount(ctx, sqlc.InsertBankAccountParams{CompanyID: b.CompanyID, SupplierID: b.SupplierID, BankName: b.BankName,
		BankCode: b.BankCode, AccountNumber: b.AccountNumber, AccountHolder: b.AccountHolder, IsDefault: b.IsDefault, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt})
}

func (r *Repository) UpdateBankAccount(ctx context.Context, b domain.BankAccount) error {
	return r.q(ctx).UpdateBankAccount(ctx, sqlc.UpdateBankAccountParams{BankName: b.BankName, BankCode: b.BankCode, AccountNumber: b.AccountNumber,
		AccountHolder: b.AccountHolder, IsDefault: b.IsDefault, UpdatedAt: b.UpdatedAt, CompanyID: b.CompanyID, SupplierID: b.SupplierID, ID: b.ID})
}

func (r *Repository) SetBankAccountActive(ctx context.Context, company, supplier, id int64, active bool, at time.Time) error {
	return r.q(ctx).SetBankAccountActive(ctx, sqlc.SetBankAccountActiveParams{IsActive: active, UpdatedAt: at, CompanyID: company, SupplierID: supplier, ID: id})
}

func (r *Repository) ClearDefaultBankAccount(ctx context.Context, company, supplier, except int64) error {
	return r.q(ctx).ClearDefaultBankAccount(ctx, sqlc.ClearDefaultBankAccountParams{CompanyID: company, SupplierID: supplier, ID: except})
}

func toContact(row sqlc.SuppContact) domain.Contact {
	return domain.Contact{ID: row.ID, CompanyID: row.CompanyID, SupplierID: row.SupplierID, Name: row.Name, Title: row.Title,
		Email: row.Email, Phone: row.Phone, IsPrimary: row.IsPrimary, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func (r *Repository) Contacts(ctx context.Context, company, supplier int64) ([]domain.Contact, error) {
	rows, err := r.q(ctx).ListContacts(ctx, sqlc.ListContactsParams{CompanyID: company, SupplierID: supplier})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Contact, len(rows))
	for i, row := range rows {
		out[i] = toContact(row)
	}
	return out, nil
}

func (r *Repository) Contact(ctx context.Context, company, supplier, id int64) (domain.Contact, error) {
	row, err := r.q(ctx).GetContact(ctx, sqlc.GetContactParams{CompanyID: company, SupplierID: supplier, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Contact{}, application.ErrNotFound
	}
	return toContact(row), err
}

func (r *Repository) InsertContact(ctx context.Context, c domain.Contact) (int64, error) {
	return r.q(ctx).InsertContact(ctx, sqlc.InsertContactParams{CompanyID: c.CompanyID, SupplierID: c.SupplierID, Name: c.Name, Title: c.Title,
		Email: c.Email, Phone: c.Phone, IsPrimary: c.IsPrimary, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt})
}

func (r *Repository) UpdateContact(ctx context.Context, c domain.Contact) error {
	return r.q(ctx).UpdateContact(ctx, sqlc.UpdateContactParams{Name: c.Name, Title: c.Title, Email: c.Email, Phone: c.Phone, IsPrimary: c.IsPrimary,
		UpdatedAt: c.UpdatedAt, CompanyID: c.CompanyID, SupplierID: c.SupplierID, ID: c.ID})
}

func (r *Repository) DeleteContact(ctx context.Context, company, supplier, id int64) error {
	n, err := r.q(ctx).DeleteContact(ctx, sqlc.DeleteContactParams{CompanyID: company, SupplierID: supplier, ID: id})
	if err == nil && n == 0 {
		return application.ErrNotFound
	}
	return err
}

func (r *Repository) ClearPrimaryContact(ctx context.Context, company, supplier, except int64) error {
	return r.q(ctx).ClearPrimaryContact(ctx, sqlc.ClearPrimaryContactParams{CompanyID: company, SupplierID: supplier, ID: except})
}

func (r *Repository) Settings(ctx context.Context, company int64) (application.Settings, error) {
	row, err := r.q(ctx).GetSettings(ctx, company)
	if errors.Is(err, sql.ErrNoRows) {
		return application.Settings{CodePrefix: application.DefaultPrefix}, nil
	}
	return application.Settings{CodePrefix: row.CodePrefix, LastNumber: row.LastNumber}, err
}

func (r *Repository) LockSettings(ctx context.Context, company int64, at time.Time) (application.Settings, error) {
	if err := r.q(ctx).EnsureSettings(ctx, sqlc.EnsureSettingsParams{CompanyID: company, UpdatedAt: at}); err != nil {
		return application.Settings{}, err
	}
	row, err := r.q(ctx).LockSettings(ctx, company)
	return application.Settings{CodePrefix: row.CodePrefix, LastNumber: row.LastNumber}, err
}

func (r *Repository) SetLastNumber(ctx context.Context, company, n int64) error {
	return r.q(ctx).SetLastNumber(ctx, sqlc.SetLastNumberParams{LastNumber: n, CompanyID: company})
}

func (r *Repository) SetPrefix(ctx context.Context, company int64, prefix string, at time.Time) error {
	return r.q(ctx).SetPrefix(ctx, sqlc.SetPrefixParams{CodePrefix: prefix, UpdatedAt: at, CompanyID: company})
}
