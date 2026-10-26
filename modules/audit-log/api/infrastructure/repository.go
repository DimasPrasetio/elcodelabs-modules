// Package infrastructure stores the audit trail in MySQL through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"time"

	"__GO_MODULE__/internal/modules/audit-log/application"
	"__GO_MODULE__/internal/modules/audit-log/domain"
	"__GO_MODULE__/internal/modules/audit-log/infrastructure/sqlc"
	"__GO_MODULE__/internal/shared/pagination"
)

var _ application.Repository = (*Repository)(nil)

// Repository implements application.Repository.
type Repository struct {
	q *sqlc.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{q: sqlc.New(db)}
}

func (r *Repository) Insert(ctx context.Context, e domain.Entry) error {
	return r.q.InsertEntry(ctx, sqlc.InsertEntryParams{
		OccurredAt: e.OccurredAt, ActorID: sql.NullInt64{Int64: e.ActorID, Valid: e.ActorID != 0},
		Module: e.Module, Action: e.Action, Target: e.Target, Summary: e.Summary, Data: e.Data, Ip: e.IP, RequestID: e.RequestID,
	})
}

func (r *Repository) List(ctx context.Context, f application.Filter, p pagination.Params) ([]domain.Entry, int64, error) {
	actor := sql.NullInt64{Int64: f.Actor, Valid: true}
	rows, err := r.q.ListEntries(ctx, sqlc.ListEntriesParams{Module: f.Module, Actor: actor, Target: f.Target, FromAt: f.From, ToAt: f.To,
		Limit: int32(p.Limit), Offset: int32(p.Offset())})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountEntries(ctx, sqlc.CountEntriesParams{Module: f.Module, Actor: actor, Target: f.Target, FromAt: f.From, ToAt: f.To})
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Entry, len(rows))
	for i, row := range rows {
		out[i] = domain.Entry{ID: row.ID, OccurredAt: row.OccurredAt, ActorID: row.ActorID.Int64, Module: row.Module, Action: row.Action,
			Target: row.Target, Summary: row.Summary, Data: row.Data, IP: row.Ip, RequestID: row.RequestID}
	}
	return out, total, nil
}

func (r *Repository) Modules(ctx context.Context) ([]string, error) { return r.q.DistinctModules(ctx) }

func (r *Repository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	return r.q.DeleteOlderThan(ctx, before)
}
