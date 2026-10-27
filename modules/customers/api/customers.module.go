// Package customers keeps a company's customers: identity, tax identity as Coretax needs it,
// billing and shipping addresses, contacts, payment terms, a credit limit, and whether they are
// on hold — deactivated, never deleted. It offers contracts.CustomerClient, needs no port, and
// imports no module.
package customers

import (
	"context"
	"errors"
	"net/http"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/customers/application"
	"__GO_MODULE__/internal/modules/customers/contracts"
	"__GO_MODULE__/internal/modules/customers/domain"
	"__GO_MODULE__/internal/modules/customers/infrastructure"
	"__GO_MODULE__/internal/modules/customers/presentation"
)

// Module is the customers module.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// New builds the module.
func New(d *kernel.Deps) (*Module, error) {
	d.Authz.Declare(
		kernel.Permission{Name: application.PermRead, Module: "customers", Description: "Melihat pelanggan"},
		kernel.Permission{Name: application.PermManage, Module: "customers", Description: "Menambah, mengubah, menahan, dan menonaktifkan pelanggan"},
		kernel.Permission{Name: application.PermCredit, Module: "customers", Description: "Mengubah termin pembayaran dan batas kredit pelanggan"},
		kernel.Permission{Name: application.PermSensitive, Module: "customers", Description: "Melihat NIK dan nomor paspor pelanggan secara utuh"},
	)
	svc := application.NewService(infrastructure.NewRepository(d.Tx), d.Tx, d.Audit, d.Authz, d.Auth, d.Clock)
	return &Module{svc: svc, handler: presentation.NewHandler(svc, d.Authz)}, nil
}

// Routes mounts /api/v1/customers.
func (m *Module) Routes(mux *http.ServeMux) { m.handler.Routes(mux) }

// Client returns the module's public contract.
func (m *Module) Client() contracts.CustomerClient { return client{m.svc} }

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

func toContract(c domain.Customer) contracts.Customer {
	return contracts.Customer{ID: c.ID, Code: c.Code, Name: c.Name, PartyType: string(c.PartyType), PaymentDays: c.PaymentDays,
		CreditLimit: c.CreditLimit, OnHold: c.OnHold, HoldReason: c.HoldReason, IsActive: c.IsActive}
}

func (c client) Get(ctx context.Context, id int64) (contracts.Customer, error) {
	cu, err := c.svc.Get(ctx, id)
	if err != nil {
		return contracts.Customer{}, contractErr(err)
	}
	return toContract(cu), nil
}

func (c client) Search(ctx context.Context, q string, limit int) ([]contracts.Customer, error) {
	rows, err := c.svc.Search(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.Customer, len(rows))
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
	s := contracts.Snapshot{CustomerID: cu.ID, Code: cu.Code, Name: cu.InvoiceName(), PartyType: string(cu.PartyType), IsForeign: cu.IsForeign,
		Country: cu.Country, TaxIDKind: string(cu.TaxIDKind), TaxID: cu.TaxID, IsPKP: cu.IsPKP, VATCollector: string(cu.VATCollector), Email: cu.Email}
	if a.ID != 0 {
		s.AddressID, s.Address, s.NITKU = a.ID, a.OneLine(), a.NITKU
	}
	return s, nil
}
