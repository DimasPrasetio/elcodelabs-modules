package modules

import (
	"context"
	"errors"

	"__GO_MODULE__/internal/kernel"
	fixinvoiceports "__GO_MODULE__/internal/modules/fixinvoice/ports"
	periodcontracts "__GO_MODULE__/internal/modules/period/contracts"
)

// fixinvoicePeriods satisfies the fixture's Periods port with period's client. Mapping only.
type fixinvoicePeriods struct{ period periodcontracts.PeriodClient }

func (a fixinvoicePeriods) Declare(area, label string) { a.period.Declare(area, label) }

func (a fixinvoicePeriods) EnsureOpen(ctx context.Context, area string, date kernel.Date) error {
	err := a.period.EnsureOpen(ctx, area, date)
	if errors.Is(err, periodcontracts.ErrClosed) {
		return &fixinvoiceports.PeriodClosedError{Message: err.Error()}
	}
	return err
}
