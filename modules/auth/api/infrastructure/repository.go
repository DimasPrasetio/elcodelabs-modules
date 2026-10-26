// Package infrastructure stores sessions in MySQL through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"__GO_MODULE__/internal/modules/auth/application"
	"__GO_MODULE__/internal/modules/auth/infrastructure/sqlc"
)

// Sessions implements application.Sessions.
type Sessions struct {
	q *sqlc.Queries
}

func NewSessions(db *sql.DB) *Sessions {
	return &Sessions{q: sqlc.New(db)}
}

func (s *Sessions) Create(ctx context.Context, r application.SessionRecord) error {
	return s.q.CreateSession(ctx, sqlc.CreateSessionParams{
		ID: r.ID, UserID: r.UserID, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, Ip: r.IP, UserAgent: r.UserAgent,
	})
}

func (s *Sessions) Get(ctx context.Context, id string) (application.SessionRecord, error) {
	row, err := s.q.GetSession(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return application.SessionRecord{}, application.ErrNoSession
	}
	if err != nil {
		return application.SessionRecord{}, err
	}
	return application.SessionRecord{ID: row.ID, UserID: row.UserID, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, Revoked: row.RevokedAt.Valid}, nil
}

func (s *Sessions) Revoke(ctx context.Context, id string, at time.Time) error {
	return s.q.RevokeSession(ctx, sqlc.RevokeSessionParams{RevokedAt: sql.NullTime{Time: at, Valid: true}, ID: id})
}

func (s *Sessions) DeleteExpired(ctx context.Context, before time.Time) error {
	return s.q.DeleteExpiredSessions(ctx, before)
}
