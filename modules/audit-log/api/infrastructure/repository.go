// Package infrastructure stores the audit trail in MySQL through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/audit-log/application"
	"__GO_MODULE__/internal/modules/audit-log/domain"
	"__GO_MODULE__/internal/modules/audit-log/infrastructure/sqlc"
	"__GO_MODULE__/internal/shared/pagination"
)

var _ application.Repository = (*Repository)(nil)

// Repository implements application.Repository.
type Repository struct {
	tx *kernel.TxSlot
}

func NewRepository(tx *kernel.TxSlot) *Repository {
	return &Repository{tx: tx}
}

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func (r *Repository) Insert(ctx context.Context, e domain.Entry) error {
	return r.q(ctx).InsertEntry(ctx, sqlc.InsertEntryParams{
		OccurredAt: e.OccurredAt, ActorID: sql.NullInt64{Int64: e.ActorID, Valid: e.ActorID != 0},
		Kind: e.Kind, CompanyID: e.CompanyID, BranchID: e.BranchID, Module: e.Module, Action: e.Action, Target: e.Target, Summary: e.Summary, Data: e.Data, Ip: e.IP, RequestID: e.RequestID,
	})
}

func (r *Repository) List(ctx context.Context, f application.Filter, p pagination.Params) ([]domain.Entry, int64, error) {
	actor := sql.NullInt64{Int64: f.Actor, Valid: true}
	rows, err := r.q(ctx).ListEntries(ctx, sqlc.ListEntriesParams{CompanyID: f.Company, Kind: f.Kind, Module: f.Module, Actor: actor, Target: f.Target,
		FromAt: f.From, ToAt: f.To, Limit: int32(p.Limit), Offset: int32(p.Offset())})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q(ctx).CountEntries(ctx, sqlc.CountEntriesParams{CompanyID: f.Company, Kind: f.Kind, Module: f.Module, Actor: actor, Target: f.Target,
		FromAt: f.From, ToAt: f.To})
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Entry, len(rows))
	for i, row := range rows {
		out[i] = domain.Entry{ID: row.ID, OccurredAt: row.OccurredAt, ActorID: row.ActorID.Int64, Kind: row.Kind, CompanyID: f.Company, BranchID: row.BranchID,
			Module: row.Module, Action: row.Action,
			Target: row.Target, Summary: row.Summary, Data: row.Data, IP: row.Ip, RequestID: row.RequestID}
	}
	return out, total, nil
}

func (r *Repository) Modules(ctx context.Context, company int64) ([]string, error) {
	return r.q(ctx).DistinctModules(ctx, company)
}

func (r *Repository) SystemDeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	return r.q(ctx).SystemDeleteOlderThan(ctx, before)
}
