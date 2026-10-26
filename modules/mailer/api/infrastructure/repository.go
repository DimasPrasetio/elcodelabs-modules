// Package infrastructure stores the outbox in MySQL (the module's own sqlc package) and hands
// messages to a mail server.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/mailer/application"
	"__GO_MODULE__/internal/modules/mailer/domain"
	"__GO_MODULE__/internal/modules/mailer/infrastructure/sqlc"
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

func (r *Repository) Enqueue(ctx context.Context, m domain.Message) (int64, error) {
	return r.q(ctx).Enqueue(ctx, sqlc.EnqueueParams{
		ToAddress: m.To, ToName: m.ToName, Subject: m.Subject, TextBody: m.Text, HtmlBody: m.HTML,
		NextAttemptAt: m.NextAttemptAt, CreatedAt: m.CreatedAt,
	})
}

func (r *Repository) Get(ctx context.Context, id int64) (domain.Message, error) {
	row, err := r.q(ctx).GetMessage(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Message{}, application.ErrNotFound
	}
	if err != nil {
		return domain.Message{}, err
	}
	return domain.Message{
		ID: row.ID, To: row.ToAddress, ToName: row.ToName, Subject: row.Subject, Text: row.TextBody, HTML: row.HtmlBody,
		Status: row.Status, Attempts: int(row.Attempts), NextAttemptAt: row.NextAttemptAt, LastError: row.LastError,
		CreatedAt: row.CreatedAt, SentAt: row.SentAt.Time,
	}, nil
}

func (r *Repository) ClaimAttempt(ctx context.Context, id int64, attempt int, now, until time.Time) (bool, error) {
	n, err := r.q(ctx).ClaimAttempt(ctx, sqlc.ClaimAttemptParams{
		Now: now, LockedUntil: sql.NullTime{Time: until, Valid: true}, ID: id, AttemptsMade: int32(attempt - 1),
		FreeBefore: sql.NullTime{Time: now, Valid: true},
	})
	return n == 1, err
}

func (r *Repository) ReleaseAttempt(ctx context.Context, id int64, attempt int) error {
	return r.q(ctx).ReleaseAttempt(ctx, sqlc.ReleaseAttemptParams{ID: id, Attempts: int32(attempt - 1)})
}

func (r *Repository) Postpone(ctx context.Context, id int64, attempt int, until, staleBefore time.Time) (bool, error) {
	n, err := r.q(ctx).Postpone(ctx, sqlc.PostponeParams{Due: until, ID: id, AttemptsMade: int32(attempt - 1), StaleBefore: staleBefore})
	return n == 1, err
}

func (r *Repository) MarkSent(ctx context.Context, id int64, attempts int, at time.Time) error {
	return r.q(ctx).MarkSent(ctx, sqlc.MarkSentParams{Attempts: int32(attempts), SentAt: sql.NullTime{Time: at, Valid: true}, ID: id})
}

func (r *Repository) MarkFailedAttempt(ctx context.Context, id int64, status string, attempts int, next time.Time, lastErr string) error {
	return r.q(ctx).MarkFailedAttempt(ctx, sqlc.MarkFailedAttemptParams{Status: status, Attempts: int32(attempts), NextAttemptAt: next, LastError: lastErr, ID: id})
}

func (r *Repository) Stale(ctx context.Context, before time.Time) ([]domain.Message, error) {
	rows, err := r.q(ctx).Stale(ctx, before)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Message, len(rows))
	for i, row := range rows {
		out[i] = domain.Message{ID: row.ID, Attempts: int(row.Attempts)}
	}
	return out, nil
}

func (r *Repository) Requeue(ctx context.Context, id int64, now time.Time) (bool, error) {
	n, err := r.q(ctx).Requeue(ctx, sqlc.RequeueParams{NextAttemptAt: now, ID: id})
	return n > 0, err
}

func (r *Repository) Prune(ctx context.Context, sentBefore, failedBefore time.Time) (int64, error) {
	return r.q(ctx).Prune(ctx, sqlc.PruneParams{SentBefore: sentBefore, FailedBefore: failedBefore})
}

func (r *Repository) List(ctx context.Context, status, search string, p pagination.Params) ([]domain.Message, int64, error) {
	rows, err := r.q(ctx).ListMessages(ctx, sqlc.ListMessagesParams{Status: status, Search: search, Limit: int32(p.Limit), Offset: int32(p.Offset())})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q(ctx).CountMessages(ctx, sqlc.CountMessagesParams{Status: status, Search: search})
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Message, len(rows))
	for i, row := range rows {
		out[i] = domain.Message{
			ID: row.ID, To: row.ToAddress, ToName: row.ToName, Subject: row.Subject, Status: row.Status,
			Attempts: int(row.Attempts), NextAttemptAt: row.NextAttemptAt, LastError: row.LastError,
			CreatedAt: row.CreatedAt, SentAt: row.SentAt.Time,
		}
	}
	return out, total, nil
}
