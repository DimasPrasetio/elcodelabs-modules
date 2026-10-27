package modules

import (
	"context"
	"errors"

	customerscontracts "__GO_MODULE__/internal/modules/customers/contracts"
	fixinvoiceports "__GO_MODULE__/internal/modules/fixinvoice/ports"
)

// fixinvoiceCustomers satisfies the fixture's Customers port with the customers client. Mapping only.
type fixinvoiceCustomers struct {
	customers customerscontracts.CustomerClient
}

func (a fixinvoiceCustomers) Usable(ctx context.Context, id int64) error {
	err := a.customers.Usable(ctx, id)
	switch {
	case errors.Is(err, customerscontracts.ErrNotFound):
		return &fixinvoiceports.CustomerRefusedError{Message: "Pelanggan tidak ditemukan"}
	case errors.Is(err, customerscontracts.ErrInactive):
		return &fixinvoiceports.CustomerRefusedError{Message: "Pelanggan nonaktif"}
	case errors.Is(err, customerscontracts.ErrOnHold):
		return &fixinvoiceports.CustomerRefusedError{Message: "Pelanggan sedang ditahan"}
	}
	return err
}

func (a fixinvoiceCustomers) Snapshot(ctx context.Context, id int64) (fixinvoiceports.CustomerSnapshot, error) {
	s, err := a.customers.Snapshot(ctx, id, 0)
	return fixinvoiceports.CustomerSnapshot{Name: s.Name, TaxID: s.TaxID}, err
}
