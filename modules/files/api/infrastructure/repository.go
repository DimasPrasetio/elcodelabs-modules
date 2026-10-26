// Package infrastructure stores the files table in MySQL (the module's own sqlc package) and
// the bytes on disk.
package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"__GO_MODULE__/internal/modules/files/application"
	"__GO_MODULE__/internal/modules/files/contracts"
	"__GO_MODULE__/internal/modules/files/domain"
	"__GO_MODULE__/internal/modules/files/infrastructure/sqlc"
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

func (r *Repository) Create(ctx context.Context, f domain.Stored) (int64, error) {
	return r.q.CreateFile(ctx, sqlc.CreateFileParams{
		Owner: f.Owner, Name: f.Name, ContentType: f.ContentType, Size: f.Size, Sha256: f.SHA256, StorageKey: f.Key,
		CreatedAt: f.CreatedAt, CreatedBy: sql.NullInt64{Int64: f.CreatedBy, Valid: f.CreatedBy != 0},
	})
}

func (r *Repository) Get(ctx context.Context, id int64) (domain.Stored, error) {
	row, err := r.q.GetFile(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Stored{}, contracts.ErrNotFound
	}
	if err != nil {
		return domain.Stored{}, err
	}
	return domain.Stored{ID: row.ID, Owner: row.Owner, Name: row.Name, ContentType: row.ContentType, Size: row.Size, SHA256: row.Sha256,
		Key: row.StorageKey, CreatedAt: row.CreatedAt, CreatedBy: row.CreatedBy.Int64}, nil
}

func (r *Repository) SoftDelete(ctx context.Context, id int64, at time.Time) (bool, error) {
	n, err := r.q.SoftDeleteFile(ctx, sqlc.SoftDeleteFileParams{DeletedAt: sql.NullTime{Time: at, Valid: true}, ID: id})
	return n > 0, err
}

func (r *Repository) List(ctx context.Context, owner, search string, p pagination.Params) ([]domain.Stored, int64, error) {
	rows, err := r.q.ListFiles(ctx, sqlc.ListFilesParams{Owner: owner, Search: search, Limit: int32(p.Limit), Offset: int32(p.Offset())})
	if err != nil {
		return nil, 0, err
	}
	total, err := r.q.CountFiles(ctx, sqlc.CountFilesParams{Owner: owner, Search: search})
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Stored, len(rows))
	for i, row := range rows {
		out[i] = domain.Stored{ID: row.ID, Owner: row.Owner, Name: row.Name, ContentType: row.ContentType, Size: row.Size, SHA256: row.Sha256,
			Key: row.StorageKey, CreatedAt: row.CreatedAt, CreatedBy: row.CreatedBy.Int64}
	}
	return out, total, nil
}

func (r *Repository) PurgeCandidates(ctx context.Context, before time.Time) ([]application.Purgeable, error) {
	rows, err := r.q.PurgeCandidates(ctx, sql.NullTime{Time: before, Valid: true})
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
	return r.q.DeleteFileRow(ctx, id)
}
