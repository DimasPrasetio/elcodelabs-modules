// Package infrastructure stores formats and counters in MySQL through the module's own sqlc
// package.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/numbering/application"
	"__GO_MODULE__/internal/modules/numbering/domain"
	"__GO_MODULE__/internal/modules/numbering/infrastructure/sqlc"
)

// Repository implements application.Repository.
type Repository struct {
	tx *kernel.TxSlot
}

func NewRepository(tx *kernel.TxSlot) *Repository { return &Repository{tx: tx} }

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func (r *Repository) Formats(ctx context.Context, companyID int64) (map[string]application.StoredFormat, error) {
	rows, err := r.q(ctx).Formats(ctx, companyID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]application.StoredFormat, len(rows))
	for _, row := range rows {
		out[row.DocType] = application.StoredFormat{
			Format:    domain.Format{Pattern: row.Pattern, Reset: domain.Reset(row.Reset), PerBranch: row.PerBranch},
			UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt,
		}
	}
	return out, nil
}

func (r *Repository) FormatOf(ctx context.Context, companyID int64, docType string) (domain.Format, bool, error) {
	row, err := r.q(ctx).FormatOf(ctx, sqlc.FormatOfParams{CompanyID: companyID, DocType: docType})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Format{}, false, nil
	}
	if err != nil {
		return domain.Format{}, false, err
	}
	return domain.Format{Pattern: row.Pattern, Reset: domain.Reset(row.Reset), PerBranch: row.PerBranch}, true, nil
}

func (r *Repository) FormatOfShared(ctx context.Context, companyID int64, docType string) (domain.Format, bool, error) {
	row, err := r.q(ctx).FormatOfShared(ctx, sqlc.FormatOfSharedParams{CompanyID: companyID, DocType: docType})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Format{}, false, nil
	}
	if err != nil {
		return domain.Format{}, false, err
	}
	return domain.Format{Pattern: row.Pattern, Reset: domain.Reset(row.Reset), PerBranch: row.PerBranch}, true, nil
}

func (r *Repository) PutFormat(ctx context.Context, companyID int64, docType string, f domain.Format, by int64, at time.Time) error {
	return r.q(ctx).PutFormat(ctx, sqlc.PutFormatParams{CompanyID: companyID, DocType: docType, Pattern: f.Pattern, Reset: string(f.Reset), PerBranch: f.PerBranch, UpdatedBy: by, UpdatedAt: at})
}

func (r *Repository) DeleteFormat(ctx context.Context, companyID int64, docType string) error {
	return r.q(ctx).DeleteFormat(ctx, sqlc.DeleteFormatParams{CompanyID: companyID, DocType: docType})
}

func (r *Repository) Take(ctx context.Context, s application.Series) (int64, error) {
	if err := r.q(ctx).Bump(ctx, sqlc.BumpParams{CompanyID: s.CompanyID, DocType: s.DocType, BranchID: s.BranchID, PeriodKey: s.PeriodKey}); err != nil {
		return 0, err
	}
	return r.q(ctx).LastValueForUpdate(ctx, sqlc.LastValueForUpdateParams{CompanyID: s.CompanyID, DocType: s.DocType, BranchID: s.BranchID, PeriodKey: s.PeriodKey})
}

func (r *Repository) LastValue(ctx context.Context, s application.Series) (int64, error) {
	v, err := r.q(ctx).LastValue(ctx, sqlc.LastValueParams{CompanyID: s.CompanyID, DocType: s.DocType, BranchID: s.BranchID, PeriodKey: s.PeriodKey})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return v, err
}

func (r *Repository) LockSeries(ctx context.Context, s application.Series) (int64, error) {
	if err := r.q(ctx).EnsureSeries(ctx, sqlc.EnsureSeriesParams{CompanyID: s.CompanyID, DocType: s.DocType, BranchID: s.BranchID, PeriodKey: s.PeriodKey}); err != nil {
		return 0, err
	}
	return r.q(ctx).LastValueForUpdate(ctx, sqlc.LastValueForUpdateParams{CompanyID: s.CompanyID, DocType: s.DocType, BranchID: s.BranchID, PeriodKey: s.PeriodKey})
}

func (r *Repository) SetLastValue(ctx context.Context, s application.Series, v int64) error {
	return r.q(ctx).SetLastValue(ctx, sqlc.SetLastValueParams{CompanyID: s.CompanyID, DocType: s.DocType, BranchID: s.BranchID, PeriodKey: s.PeriodKey, LastNumber: v})
}

func (r *Repository) Issued(ctx context.Context, companyID int64, docType string) (bool, error) {
	n, err := r.q(ctx).IssuedShared(ctx, sqlc.IssuedSharedParams{CompanyID: companyID, DocType: docType})
	return n > 0, err
}

func (r *Repository) Record(ctx context.Context, companyID int64, docType, number string) (bool, error) {
	n, err := r.q(ctx).RecordIssued(ctx, sqlc.RecordIssuedParams{CompanyID: companyID, DocType: docType, Number: number})
	return n == 1, err
}

func (r *Repository) Given(ctx context.Context, companyID int64, docType, number string) (bool, error) {
	n, err := r.q(ctx).IsIssued(ctx, sqlc.IsIssuedParams{CompanyID: companyID, DocType: docType, Number: number})
	return n > 0, err
}
