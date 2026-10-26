package modules

import (
	"context"
	"errors"

	ledgercontracts "__GO_MODULE__/internal/modules/ledger/contracts"
	ordersports "__GO_MODULE__/internal/modules/orders/ports"
)

// ordersStock satisfies orders' Stock port with the ledger's client. Mapping only.
type ordersStock struct{ ledger ledgercontracts.StockClient }

func (a ordersStock) Take(ctx context.Context, sku string, qty int) error {
	err := a.ledger.Take(ctx, sku, qty)
	if errors.Is(err, ledgercontracts.ErrInsufficient) {
		return ordersports.ErrInsufficient
	}
	return err
}
