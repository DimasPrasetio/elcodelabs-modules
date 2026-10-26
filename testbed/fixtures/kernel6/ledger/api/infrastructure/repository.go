// Package infrastructure stores the ledger through its sqlc package, inside d.Tx.
package infrastructure

import (
	"context"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/ledger/infrastructure/sqlc"
)

type Repository struct{ tx *kernel.TxSlot }

func NewRepository(tx *kernel.TxSlot) *Repository { return &Repository{tx: tx} }

func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func company(ctx context.Context) int64 { return kernel.ScopeFrom(ctx).CompanyID }

func (r *Repository) Add(ctx context.Context, sku string, qty int) error {
	return r.q(ctx).AddStock(ctx, sqlc.AddStockParams{CompanyID: company(ctx), Sku: sku, Qty: int32(qty)})
}

func (r *Repository) Take(ctx context.Context, sku string, qty int) (bool, error) {
	n, err := r.q(ctx).TakeStock(ctx, sqlc.TakeStockParams{Take: int32(qty), CompanyID: company(ctx), Sku: sku})
	return n > 0, err
}

func (r *Repository) Note(ctx context.Context, body string) error {
	return r.q(ctx).AddNote(ctx, sqlc.AddNoteParams{CompanyID: company(ctx), Body: body})
}
