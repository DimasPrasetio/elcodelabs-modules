// Package infrastructure stores settings in MySQL through the module's own sqlc package.
package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/settings/application"
	"__GO_MODULE__/internal/modules/settings/infrastructure/sqlc"
)

var _ application.Repository = (*Repository)(nil)

// Repository implements application.Repository. Every call works in the ctx's company.
type Repository struct {
	tx *kernel.TxSlot
}

func NewRepository(tx *kernel.TxSlot) *Repository {
	return &Repository{tx: tx}
}

// q runs on the transaction in ctx when there is one (d.Tx), else on the pool.
func (r *Repository) q(ctx context.Context) *sqlc.Queries { return sqlc.New(r.tx.Conn(ctx)) }

func company(ctx context.Context) int64 { return kernel.ScopeFrom(ctx).CompanyID }

func (r *Repository) Get(ctx context.Context, name string, dst any) (bool, error) {
	row, err := r.q(ctx).GetSetting(ctx, sqlc.GetSettingParams{CompanyID: company(ctx), Name: name})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(row.Value, dst)
}

// Update reads, changes, and writes the setting under a row lock, inside d.Tx (a caller's
// transaction is joined).
func (r *Repository) Update(ctx context.Context, name string, by int64, dst any, change func() error) error {
	// The row exists before it is locked: locking a missing row takes a gap lock, and two first
	// saves holding one each deadlock on the insert.
	if err := r.q(ctx).EnsureSetting(ctx, sqlc.EnsureSettingParams{CompanyID: company(ctx), Name: name, UpdatedAt: time.Now().UTC()}); err != nil {
		return err
	}
	return r.tx.Do(ctx, func(ctx context.Context) error {
		q := r.q(ctx)
		raw, err := q.GetSettingForUpdate(ctx, sqlc.GetSettingForUpdateParams{CompanyID: company(ctx), Name: name})
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
		return q.PutSetting(ctx, sqlc.PutSettingParams{CompanyID: company(ctx), Name: name, Value: value, UpdatedAt: time.Now().UTC(),
			UpdatedBy: sql.NullInt64{Int64: by, Valid: by != 0}})
	})
}
