-- name: CreateFile :execlastid
INSERT INTO files (company_id, owner, name, content_type, size, sha256, storage_key, created_at, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetFile :one
SELECT id, owner, name, content_type, size, sha256, storage_key, created_at, created_by, locked_at
FROM files WHERE id = ? AND company_id = ? AND deleted_at IS NULL;

-- name: SoftDeleteFile :execrows
UPDATE files SET deleted_at = ? WHERE id = ? AND company_id = ? AND deleted_at IS NULL AND locked_at IS NULL;

-- name: LockFile :exec
UPDATE files SET locked_at = COALESCE(locked_at, ?) WHERE id = ? AND company_id = ? AND deleted_at IS NULL;

-- name: ListFiles :many
SELECT id, owner, name, content_type, size, sha256, storage_key, created_at, created_by, locked_at
FROM files
WHERE company_id = sqlc.arg(company_id)
  AND deleted_at IS NULL
  AND (sqlc.arg(owner) = '' OR owner = sqlc.arg(owner))
  AND (sqlc.arg(search) = '' OR name LIKE CONCAT('%', sqlc.arg(search), '%'))
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountFiles :one
SELECT COUNT(*) FROM files
WHERE company_id = sqlc.arg(company_id)
  AND deleted_at IS NULL
  AND (sqlc.arg(owner) = '' OR owner = sqlc.arg(owner))
  AND (sqlc.arg(search) = '' OR name LIKE CONCAT('%', sqlc.arg(search), '%'));

-- name: SystemPurgeCandidates :many
-- all companies: purging bytes deleted 30 days ago is the app's housekeeping.
SELECT id, storage_key FROM files WHERE deleted_at < ? LIMIT 200;

-- name: SystemDeleteFileRow :exec
-- all companies: the purge removes rows it found above.
DELETE FROM files WHERE id = ?;
