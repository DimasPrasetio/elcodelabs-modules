// Package infrastructure stores the fixture's invoices and stock.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/fixinvoice/application"
	"__GO_MODULE__/internal/modules/fixinvoice/infrastructure/sqlc"
	"__GO_MODULE__/internal/shared/docstate"
	"__GO_MODULE__/internal/shared/money"
)

type Repository struct{ tx *kernel.TxSlot }

func NewRepository(tx *kernel.TxSlot) *Repository { return &Repository{tx: tx} }

func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func company(ctx context.Context) int64 { return kernel.ScopeFrom(ctx).CompanyID }

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }
func nullID(id int64) sql.NullInt64   { return sql.NullInt64{Int64: id, Valid: id != 0} }

func (r *Repository) Create(ctx context.Context, inv application.Invoice, at time.Time) (int64, error) {
	id, err := r.q(ctx).CreateInvoice(ctx, sqlc.CreateInvoiceParams{
		CompanyID: company(ctx), BranchID: inv.BranchID, DocDate: inv.Date.Start(time.UTC), Number: nullStr(inv.Number),
		Discount: inv.Discount.String(), Total: inv.Total.String(), AmendedFrom: nullID(inv.AmendedFrom), Revision: int32(inv.Revision), CreatedAt: at,
	})
	var me *mysql.MySQLError
	// An amendment created twice collides on its number or on amended_from: either way it exists.
	if errors.As(err, &me) && me.Number == 1062 && inv.AmendedFrom != 0 {
		return 0, application.ErrAmended
	}
	if err != nil {
		return 0, err
	}
	return id, r.addLines(ctx, id, inv.Lines)
}

func (r *Repository) addLines(ctx context.Context, id int64, lines []application.Line) error {
	for _, l := range lines {
		if err := r.q(ctx).AddLine(ctx, sqlc.AddLineParams{CompanyID: company(ctx), InvoiceID: id, Sku: l.SKU, Qty: l.Qty.String(), Price: l.Price.String(),
			Amount: l.Amount.String(), DiscountShare: l.DiscountShare.String()}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id int64, forUpdate bool) (application.Invoice, error) {
	var row sqlc.FixInvoice
	var err error
	if forUpdate {
		row, err = r.q(ctx).GetInvoiceForUpdate(ctx, sqlc.GetInvoiceForUpdateParams{ID: id, CompanyID: company(ctx)})
	} else {
		row, err = r.q(ctx).GetInvoice(ctx, sqlc.GetInvoiceParams{ID: id, CompanyID: company(ctx)})
	}
	if errors.Is(err, sql.ErrNoRows) {
		return application.Invoice{}, application.ErrNotFound
	}
	if err != nil {
		return application.Invoice{}, err
	}
	inv := application.Invoice{ID: row.ID, BranchID: row.BranchID, Date: kernel.DateOf(row.DocDate, time.UTC), Number: row.Number.String,
		Fields: docstate.Fields{Status: docstate.Status(row.Status), PostedAt: row.PostedAt.Time, PostedBy: row.PostedBy.Int64, CancelledAt: row.CancelledAt.Time,
			CancelledBy: row.CancelledBy.Int64, CancelReason: row.CancelReason.String, AmendedFrom: row.AmendedFrom.Int64, Revision: int(row.Revision)}}
	if inv.Discount, err = money.ParseAmount(row.Discount); err != nil {
		return inv, err
	}
	if inv.Total, err = money.ParseAmount(row.Total); err != nil {
		return inv, err
	}
	lines, err := r.q(ctx).Lines(ctx, sqlc.LinesParams{CompanyID: company(ctx), InvoiceID: id})
	if err != nil {
		return inv, err
	}
	for _, l := range lines {
		var line application.Line
		line.SKU = l.Sku
		if line.Qty, err = money.ParseQty(l.Qty); err == nil {
			if line.Price, err = money.ParseAmount(l.Price); err == nil {
				if line.Amount, err = money.ParseAmount(l.Amount); err == nil {
					line.DiscountShare, err = money.ParseAmount(l.DiscountShare)
				}
			}
		}
		if err != nil {
			return inv, err
		}
		inv.Lines = append(inv.Lines, line)
	}
	return inv, nil
}

func (r *Repository) UpdateDraft(ctx context.Context, inv application.Invoice) (int64, error) {
	n, err := r.q(ctx).UpdateDraft(ctx, sqlc.UpdateDraftParams{DocDate: inv.Date.Start(time.UTC), Discount: inv.Discount.String(), Total: inv.Total.String(), ID: inv.ID, CompanyID: company(ctx)})
	if err != nil {
		return 0, err
	}
	if err := r.q(ctx).DeleteLines(ctx, sqlc.DeleteLinesParams{CompanyID: company(ctx), InvoiceID: inv.ID}); err != nil {
		return 0, err
	}
	// An edit that changes only the lines changes no invoice column: the row was locked as a
	// draft, so it is still one.
	return max(n, 1), r.addLines(ctx, inv.ID, inv.Lines)
}

func (r *Repository) MarkPosted(ctx context.Context, id int64, number string, at time.Time, by int64) (int64, error) {
	return r.q(ctx).MarkPosted(ctx, sqlc.MarkPostedParams{Number: nullStr(number), PostedAt: sql.NullTime{Time: at, Valid: true}, PostedBy: nullID(by), ID: id, CompanyID: company(ctx)})
}

func (r *Repository) MarkCancelled(ctx context.Context, id int64, reason string, at time.Time, by int64) (int64, error) {
	return r.q(ctx).MarkCancelled(ctx, sqlc.MarkCancelledParams{CancelledAt: sql.NullTime{Time: at, Valid: true}, CancelledBy: nullID(by), CancelReason: nullStr(reason), ID: id, CompanyID: company(ctx)})
}

func (r *Repository) DeleteDraft(ctx context.Context, id int64) (int64, error) {
	if err := r.q(ctx).DeleteLines(ctx, sqlc.DeleteLinesParams{CompanyID: company(ctx), InvoiceID: id}); err != nil {
		return 0, err
	}
	return r.q(ctx).DeleteDraft(ctx, sqlc.DeleteDraftParams{ID: id, CompanyID: company(ctx)})
}

func (r *Repository) AddStock(ctx context.Context, sku string, qty money.Qty) error {
	return r.q(ctx).AddStock(ctx, sqlc.AddStockParams{CompanyID: company(ctx), Sku: sku, Qty: qty.String()})
}

func (r *Repository) TakeStock(ctx context.Context, sku string, qty money.Qty) (bool, error) {
	n, err := r.q(ctx).TakeStock(ctx, sqlc.TakeStockParams{Qty: qty.String(), CompanyID: company(ctx), Sku: sku})
	return n == 1, err
}

func (r *Repository) ReturnStock(ctx context.Context, sku string, qty money.Qty) error {
	return r.q(ctx).ReturnStock(ctx, sqlc.ReturnStockParams{Qty: qty.String(), CompanyID: company(ctx), Sku: sku})
}
