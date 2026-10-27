// Package infrastructure stores lock dates and fiscal-year settings in MySQL through the
// module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/period/domain"
	"__GO_MODULE__/internal/modules/period/infrastructure/sqlc"
)

// Repository implements application.Repository.
type Repository struct {
	tx *kernel.TxSlot
}

func NewRepository(tx *kernel.TxSlot) *Repository { return &Repository{tx: tx} }

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

// DATE columns arrive as midnight UTC (the kernel's connection runs in UTC).
func day(t time.Time) kernel.Date      { return kernel.DateOf(t, time.UTC) }
func midnight(d kernel.Date) time.Time { return d.Start(time.UTC) }

func (r *Repository) HoldShared(ctx context.Context, companyID int64) error {
	return r.q(ctx).HoldShared(ctx, companyID)
}

func (r *Repository) HoldExclusive(ctx context.Context, companyID int64) error {
	return r.q(ctx).HoldExclusive(ctx, companyID)
}

func (r *Repository) LocksOfArea(ctx context.Context, companyID int64, area string, shared bool) (map[string]kernel.Date, error) {
	out := map[string]kernel.Date{}
	if shared {
		rows, err := r.q(ctx).LocksOfAreaShared(ctx, sqlc.LocksOfAreaSharedParams{CompanyID: companyID, Area: area})
		for _, row := range rows {
			out[row.Area] = day(row.LockedThrough)
		}
		return out, err
	}
	rows, err := r.q(ctx).LocksOfArea(ctx, sqlc.LocksOfAreaParams{CompanyID: companyID, Area: area})
	for _, row := range rows {
		out[row.Area] = day(row.LockedThrough)
	}
	return out, err
}

func (r *Repository) Locks(ctx context.Context, companyID int64) ([]domain.Lock, error) {
	rows, err := r.q(ctx).Locks(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Lock, len(rows))
	for i, row := range rows {
		out[i] = domain.Lock{Area: row.Area, LockedThrough: day(row.LockedThrough), UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt}
	}
	return out, nil
}

func (r *Repository) PutLock(ctx context.Context, companyID int64, l domain.Lock) error {
	return r.q(ctx).PutLock(ctx, sqlc.PutLockParams{CompanyID: companyID, Area: l.Area, LockedThrough: midnight(l.LockedThrough), UpdatedBy: l.UpdatedBy, UpdatedAt: l.UpdatedAt})
}

func (r *Repository) DeleteLock(ctx context.Context, companyID int64, area string) error {
	return r.q(ctx).DeleteLock(ctx, sqlc.DeleteLockParams{CompanyID: companyID, Area: area})
}

func (r *Repository) FiscalStart(ctx context.Context, companyID int64) (int, error) {
	m, err := r.q(ctx).FiscalStart(ctx, companyID)
	if errors.Is(err, sql.ErrNoRows) {
		return 1, nil
	}
	return int(m), err
}

func (r *Repository) SetFiscalStart(ctx context.Context, companyID int64, month int, at time.Time) error {
	return r.q(ctx).SetFiscalStart(ctx, sqlc.SetFiscalStartParams{CompanyID: companyID, FiscalYearStartMonth: int8(month), UpdatedAt: sql.NullTime{Time: at, Valid: true}})
}
