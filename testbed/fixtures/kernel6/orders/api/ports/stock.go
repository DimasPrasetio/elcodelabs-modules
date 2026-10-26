// Package ports is what orders needs from outside.
package ports

import (
	"context"
	"errors"
)

// ErrInsufficient means the stock cannot cover the order.
var ErrInsufficient = errors.New("insufficient stock")

// Stock takes stock — inside the order's transaction.
type Stock interface {
	Take(ctx context.Context, sku string, qty int) error
}
