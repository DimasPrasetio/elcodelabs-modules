// Package infrastructure stores the outbox in MySQL (the module's own sqlc package) and hands
// messages to a mail server.
package infrastructure

import (
	"context"
	"database/sql"
	"time"

	"__GO_MODULE__/internal/modules/mailer/application"
	"__GO_MODULE__/internal/modules/mailer/domain"
	"__GO_MODULE__/internal/modules/mailer/infrastructure/sqlc"
	"__GO_MODULE__/internal/shared/pagination"
)

var _ application.Repository = (*Repository)(nil)

// Repository implements application.Repository.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

func (r *Repository) Enqueue(ctx context.Context, m domain.Message) (int64, error) {
	return r.q.Enqueue(ctx, sqlc.EnqueueParams{
		ToAddress: m.To, ToName: m.ToName, Subject: m.Subject, TextBody: m.Text, HtmlBody: m.HTML,
		NextAttemptAt: m.NextAttemptAt, CreatedAt: m.CreatedAt,
	})
}

// Claim picks due messages with FOR UPDATE SKIP LOCKED and marks them sending in one
// transaction, so two runs (two containers during a deploy) never take the same message.
func (r *Repository) Claim(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]domain.Message, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit
	q := r.q.WithTx(tx)
	ids, err := q.DueIDs(ctx, sqlc.DueIDsParams{NextAttemptAt: now, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err := q.MarkSending(ctx, sqlc.MarkSendingParams{LockedUntil: sql.NullTime{Time: now.Add(lease), Valid: true}, ID: id}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	out := make([]domain.Message, 0, len(ids))
	for _, id := range ids {
		row, err := r.q.GetMessage(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.Message{
			ID: row.ID, To: row.ToAddress, ToName: row.ToName, Subject: row.Subject, Text: row.TextBody, HTML: row.HtmlBody,
			Status: row.Status, Attempts: int(row.Attempts), NextAttemptAt: row.NextAttemptAt, LastError: row.LastError,
			CreatedAt: row.CreatedAt, SentAt: row.SentAt.Time,
		})
	}
	return out, nil
}

func (r *Repository) MarkSent(ctx context.Context, id int64, at time.Time) error {
	return r.q.MarkSent(ctx, sqlc.MarkSentParams{SentAt: sql.NullTime{Time: at, Valid: true}, ID: id})
}

func (r *Repository) MarkFailedAttempt(ctx context.Context, id int64, status string, attempts int, next time.Time, lastErr string) error {
	return r.q.MarkFailedAttempt(ctx, sqlc.MarkFailedAttemptParams{Status: status, Attempts: int32(attempts), NextAttemptAt: next, LastError: lastErr, ID: id})
}

func (r *Repository) ReleaseStuck(ctx context.Context, now time.Time) (int64, error) {
	return r.q.ReleaseStuck(ctx, sql.NullTime{Time: now, Valid: true})
}

func (r *Repository) Requeue(ctx context.Context, id int64, now time.Time) (bool, error) {
	n, err := r.q.Requeue(ctx, sqlc.RequeueParams{NextAttemptAt: now, ID: id})
	return n > 0, err
}

func (r *Repository) Prune(ctx context.Context, sentBefore, failedBefore time.Time) (int64, error) {
	return r.q.Prune(ctx, sqlc.PruneParams{SentBefore: sentBefore, FailedBefore: failedBefore})
}

func (r *Repository) List(ctx context.Context, status, search string, p pagination.Params) ([]domain.Message, int64, error) {
	rows, err := r.q.ListMessages(ctx, sqlc.ListMessagesParams{Status: status, Search: search, Limit: int32(p.Limit), Offset: int32(p.Offset())})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountMessages(ctx, sqlc.CountMessagesParams{Status: status, Search: search})
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
