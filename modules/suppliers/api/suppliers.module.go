// Package suppliers keeps a company's suppliers: identity, tax identity as Coretax needs it,
// addresses, bank accounts, contacts, payment terms, and whether they are on hold —
// deactivated, never deleted. It offers contracts.SupplierClient, needs no port, and
// imports no module.
package suppliers

import (
	"context"
	"errors"
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/suppliers/application"
	"__GO_MODULE__/internal/modules/suppliers/contracts"
	"__GO_MODULE__/internal/modules/suppliers/domain"
	"__GO_MODULE__/internal/modules/suppliers/infrastructure"
	"__GO_MODULE__/internal/modules/suppliers/presentation"
)

// Module is the suppliers module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module.
func New(d *kernel.Deps) (*Module, error) {
	d.Authz.Declare(
		kernel.Permission{Name: application.PermRead, Module: "suppliers", Description: "Melihat pemasok"},
		kernel.Permission{Name: application.PermManage, Module: "suppliers", Description: "Menambah, mengubah, menahan, dan menonaktifkan pemasok"},
		kernel.Permission{Name: application.PermSensitive, Module: "suppliers", Description: "Melihat NIK, paspor, dan rekening pemasok secara utuh, dan mengubah rekening"},
	)
	svc := application.NewService(infrastructure.NewRepository(d.Tx), d.Tx, d.Audit, d.Authz, d.Auth, d.Clock)
	return &Module{svc: svc, handler: presentation.NewHandler(svc, d.Authz)}, nil
}

// Routes mounts /api/v1/suppliers.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Client returns the module's public contract.
func (m *Module) Client() contracts.SupplierClient { return client{m.svc} }

type client struct{ svc *application.Service }

func contractErr(err error) error {
	switch {
	case errors.Is(err, application.ErrNotFound):
		return contracts.ErrNotFound
	case errors.Is(err, application.ErrInactive):
		return contracts.ErrInactive
	case errors.Is(err, application.ErrOnHold):
		return contracts.ErrOnHold
	}
	return err
}

func toContract(c domain.Supplier) contracts.Supplier {
	return contracts.Supplier{ID: c.ID, Code: c.Code, Name: c.Name, PartyType: string(c.PartyType), IsForeign: c.IsForeign,
		TaxIDKind: string(c.TaxIDKind), IsPKP: c.IsPKP, PaymentDays: c.PaymentDays, OnHold: c.OnHold, HoldReason: c.HoldReason, IsActive: c.IsActive}
}

func (c client) Get(ctx context.Context, id int64) (contracts.Supplier, error) {
	cu, err := c.svc.Get(ctx, id)
	if err != nil {
		return contracts.Supplier{}, contractErr(err)
	}
	return toContract(cu), nil
}

func (c client) Search(ctx context.Context, q string, limit int) ([]contracts.Supplier, error) {
	rows, err := c.svc.Search(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.Supplier, len(rows))
	for i, r := range rows {
		out[i] = toContract(r)
	}
	return out, nil
}

func (c client) Usable(ctx context.Context, id int64) error {
	return contractErr(c.svc.Usable(ctx, id))
}

func (c client) Snapshot(ctx context.Context, id, addressID int64) (contracts.Snapshot, error) {
	cu, a, err := c.svc.Snapshot(ctx, id, addressID)
	if err != nil {
		return contracts.Snapshot{}, contractErr(err)
	}
	s := contracts.Snapshot{SupplierID: cu.ID, Code: cu.Code, Name: cu.InvoiceName(), PartyType: string(cu.PartyType), IsForeign: cu.IsForeign,
		Country: cu.Country, TaxIDKind: string(cu.TaxIDKind), TaxID: cu.TaxID, IsPKP: cu.IsPKP, Email: cu.Email}
	if a.ID != 0 {
		s.AddressID, s.Address, s.NITKU = a.ID, a.OneLine(), a.NITKU
	}
	return s, nil
}

func (c client) DefaultBankAccount(ctx context.Context, id int64) (contracts.BankAccount, error) {
	b, err := c.svc.DefaultBankAccount(ctx, id)
	if err != nil {
		return contracts.BankAccount{}, contractErr(err)
	}
	return contracts.BankAccount{ID: b.ID, BankName: b.BankName, BankCode: b.BankCode, AccountNumber: b.AccountNumber, AccountHolder: b.AccountHolder}, nil
}
