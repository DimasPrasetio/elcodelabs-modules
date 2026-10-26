// Package infrastructure stores the files table in MySQL (the module's own sqlc package) and
// the bytes on disk.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"__GO_MODULE__/internal/kernel"
	"__GO_MODULE__/internal/modules/files/application"
	"__GO_MODULE__/internal/modules/files/contracts"
	"__GO_MODULE__/internal/modules/files/domain"
	"__GO_MODULE__/internal/modules/files/infrastructure/sqlc"
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

// company is the company the request works in (kernel 6 scope).
func company(ctx context.Context) int64 { return kernel.ScopeFrom(ctx).CompanyID }

func (r *Repository) Create(ctx context.Context, f domain.Stored) (int64, error) {
	return r.q(ctx).CreateFile(ctx, sqlc.CreateFileParams{
		CompanyID: company(ctx), Owner: f.Owner, Name: f.Name, ContentType: f.ContentType, Size: f.Size, Sha256: f.SHA256, StorageKey: f.Key,
		CreatedAt: f.CreatedAt, CreatedBy: sql.NullInt64{Int64: f.CreatedBy, Valid: f.CreatedBy != 0},
	})
}

func (r *Repository) Get(ctx context.Context, id int64) (domain.Stored, error) {
	row, err := r.q(ctx).GetFile(ctx, sqlc.GetFileParams{ID: id, CompanyID: company(ctx)})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Stored{}, contracts.ErrNotFound
	}
	if err != nil {
		return domain.Stored{}, err
	}
	return domain.Stored{ID: row.ID, Owner: row.Owner, Name: row.Name, ContentType: row.ContentType, Size: row.Size, SHA256: row.Sha256,
		Key: row.StorageKey, CreatedAt: row.CreatedAt, CreatedBy: row.CreatedBy.Int64, Locked: row.LockedAt.Valid}, nil
}

func (r *Repository) SoftDelete(ctx context.Context, id int64, at time.Time) (bool, error) {
	n, err := r.q(ctx).SoftDeleteFile(ctx, sqlc.SoftDeleteFileParams{DeletedAt: sql.NullTime{Time: at, Valid: true}, ID: id, CompanyID: company(ctx)})
	return n > 0, err
}

func (r *Repository) Lock(ctx context.Context, id int64, at time.Time) error {
	return r.q(ctx).LockFile(ctx, sqlc.LockFileParams{LockedAt: sql.NullTime{Time: at, Valid: true}, ID: id, CompanyID: company(ctx)})
}

func (r *Repository) List(ctx context.Context, owner, search string, p pagination.Params) ([]domain.Stored, int64, error) {
	rows, err := r.q(ctx).ListFiles(ctx, sqlc.ListFilesParams{CompanyID: company(ctx), Owner: owner, Search: search, Limit: int32(p.Limit), Offset: int32(p.Offset())})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q(ctx).CountFiles(ctx, sqlc.CountFilesParams{CompanyID: company(ctx), Owner: owner, Search: search})
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Stored, len(rows))
	for i, row := range rows {
		out[i] = domain.Stored{ID: row.ID, Owner: row.Owner, Name: row.Name, ContentType: row.ContentType, Size: row.Size, SHA256: row.Sha256,
			Key: row.StorageKey, CreatedAt: row.CreatedAt, CreatedBy: row.CreatedBy.Int64, Locked: row.LockedAt.Valid}
	}
	return out, total, nil
}

func (r *Repository) SystemPurgeCandidates(ctx context.Context, before time.Time) ([]application.Purgeable, error) {
	rows, err := r.q(ctx).SystemPurgeCandidates(ctx, sql.NullTime{Time: before, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]application.Purgeable, len(rows))
	for i, row := range rows {
		out[i] = application.Purgeable{ID: row.ID, Key: row.StorageKey}
	}
	return out, nil
}

func (r *Repository) DeleteRow(ctx context.Context, id int64) error {
	return r.q(ctx).SystemDeleteFileRow(ctx, id)
}
