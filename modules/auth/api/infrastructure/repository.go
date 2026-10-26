// Package infrastructure stores sessions in MySQL through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/auth/application"
	"__GO_MODULE__/internal/modules/auth/infrastructure/sqlc"
)

var (
	_ application.Sessions = (*Sessions)(nil)
	_ application.Resets   = (*Resets)(nil)
)

// Sessions implements application.Sessions.
type Sessions struct {
	tx *kernel.TxSlot
}

func NewSessions(tx *kernel.TxSlot) *Sessions {
	return &Sessions{tx: tx}
}

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Sessions) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func (s *Sessions) Create(ctx context.Context, r application.SessionRecord) error {
	return s.q(ctx).CreateSession(ctx, sqlc.CreateSessionParams{
		ID: r.ID, UserID: r.UserID, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, Ip: r.IP, UserAgent: r.UserAgent,
	})
}

func (s *Sessions) Get(ctx context.Context, id string) (application.SessionRecord, error) {
	row, err := s.q(ctx).GetSession(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return application.SessionRecord{}, application.ErrNoSession
	}
	if err != nil {
		return application.SessionRecord{}, err
	}
	return application.SessionRecord{ID: row.ID, UserID: row.UserID, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, Revoked: row.RevokedAt.Valid}, nil
}

func (s *Sessions) Revoke(ctx context.Context, id string, at time.Time) error {
	return s.q(ctx).RevokeSession(ctx, sqlc.RevokeSessionParams{RevokedAt: sql.NullTime{Time: at, Valid: true}, ID: id})
}

func (s *Sessions) DeleteExpired(ctx context.Context, before time.Time) error {
	return s.q(ctx).DeleteExpiredSessions(ctx, before)
}

// Resets implements application.Resets.
type Resets struct {
	tx *kernel.TxSlot
}

func NewResets(tx *kernel.TxSlot) *Resets {
	return &Resets{tx: tx}
}

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Resets) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func (r *Resets) Create(ctx context.Context, rec application.ResetRecord, tokenHash, ip string) error {
	return r.q(ctx).CreateReset(ctx, sqlc.CreateResetParams{UserID: rec.UserID, TokenHash: tokenHash, Email: rec.Email, CreatedAt: rec.CreatedAt, ExpiresAt: rec.ExpiresAt, Ip: ip})
}

func (r *Resets) Find(ctx context.Context, tokenHash string) (application.ResetRecord, error) {
	row, err := r.q(ctx).GetResetByHash(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return application.ResetRecord{}, application.ErrNoReset
	}
	if err != nil {
		return application.ResetRecord{}, err
	}
	return application.ResetRecord{ID: row.ID, UserID: row.UserID, Email: row.Email, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, Used: row.UsedAt.Valid}, nil
}

func (r *Resets) Use(ctx context.Context, id int64, at time.Time) (bool, error) {
	n, err := r.q(ctx).UseReset(ctx, sqlc.UseResetParams{UsedAt: sql.NullTime{Time: at, Valid: true}, ID: id})
	return n == 1, err
}

func (r *Resets) Release(ctx context.Context, id int64, usedAt time.Time) error {
	return r.q(ctx).ReleaseReset(ctx, sqlc.ReleaseResetParams{ID: id, UsedAt: sql.NullTime{Time: usedAt, Valid: true}})
}

func (r *Resets) VoidUser(ctx context.Context, userID int64, at time.Time) error {
	return r.q(ctx).VoidUserResets(ctx, sqlc.VoidUserResetsParams{UsedAt: sql.NullTime{Time: at, Valid: true}, UserID: userID})
}

func (r *Resets) DeleteOld(ctx context.Context, before time.Time) error {
	return r.q(ctx).DeleteOldResets(ctx, before)
}
