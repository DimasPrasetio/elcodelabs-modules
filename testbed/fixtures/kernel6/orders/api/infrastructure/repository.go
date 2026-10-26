// Package infrastructure stores orders through its sqlc package, inside d.Tx.
package infrastructure

import (
	"context"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/orders/infrastructure/sqlc"
)

type Repository struct{ tx *kernel.TxSlot }

func NewRepository(tx *kernel.TxSlot) *Repository { return &Repository{tx: tx} }

func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func (r *Repository) Create(ctx context.Context, sku string, qty int, at time.Time) (int64, error) {
	return r.q(ctx).CreateOrder(ctx, sqlc.CreateOrderParams{CompanyID: kernel.ScopeFrom(ctx).CompanyID, Sku: sku, Qty: int32(qty), CreatedAt: at})
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	return r.q(ctx).CountOrders(ctx, kernel.ScopeFrom(ctx).CompanyID)
}
