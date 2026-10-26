// Package infrastructure stores settings in MySQL through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"__GO_MODULE__/internal/modules/settings/application"
	"__GO_MODULE__/internal/modules/settings/infrastructure/sqlc"
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

func (r *Repository) Get(ctx context.Context, name string, dst any) (bool, error) {
	row, err := r.q.GetSetting(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(row.Value, dst)
}

func (r *Repository) Update(ctx context.Context, name string, by int64, dst any, change func() error) error {
	// The row exists before it is locked: locking a missing row takes a gap lock, and two first
	// saves holding one each deadlock on the insert.
	if err := r.q.EnsureSetting(ctx, sqlc.EnsureSettingParams{Name: name, UpdatedAt: time.Now().UTC()}); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit
	q := r.q.WithTx(tx)
	raw, err := q.GetSettingForUpdate(ctx, name)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return err
	default:
		if err := json.Unmarshal(raw, dst); err != nil {
			return err
		}
	}
	if err := change(); err != nil {
		return err
	}
	value, err := json.Marshal(dst)
	if err != nil {
		return err
	}
	if err := q.PutSetting(ctx, sqlc.PutSettingParams{Name: name, Value: value, UpdatedAt: time.Now().UTC(), UpdatedBy: sql.NullInt64{Int64: by, Valid: by != 0}}); err != nil {
		return err
	}
	return tx.Commit()
}
