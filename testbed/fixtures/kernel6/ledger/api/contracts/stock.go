// Package contracts is what the ledger offers other modules.
package contracts

import (
	"context"
	"errors"
)

// ErrInsufficient means there is less stock than asked for.
var ErrInsufficient = errors.New("insufficient stock")

// StockClient takes stock — inside the caller's d.Tx when ctx carries one.
type StockClient interface {
	Take(ctx context.Context, sku string, qty int) error
}
