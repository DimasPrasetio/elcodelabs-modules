package modules

import (
	"context"

	"__GO_MODULE__/internal/kernel"
	fixinvoiceports "__GO_MODULE__/internal/modules/fixinvoice/ports"
	numberingcontracts "__GO_MODULE__/internal/modules/numbering/contracts"
)

// fixinvoiceNumbers satisfies the fixture's Numbers port with numbering's client. Mapping only.
type fixinvoiceNumbers struct {
	numbering numberingcontracts.NumberingClient
}

func (a fixinvoiceNumbers) Declare(docType, label string, def fixinvoiceports.NumberFormat) {
	a.numbering.Declare(docType, label, numberingcontracts.Format{Pattern: def.Pattern, Reset: def.Reset, PerBranch: def.PerBranch})
}

func (a fixinvoiceNumbers) Next(ctx context.Context, docType string, date kernel.Date) (string, error) {
	return a.numbering.Next(ctx, docType, date)
}
