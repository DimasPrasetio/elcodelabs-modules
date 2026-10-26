-- name: CreateFile :execlastid
INSERT INTO files (owner, name, content_type, size, sha256, storage_key, created_at, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetFile :one
SELECT id, owner, name, content_type, size, sha256, storage_key, created_at, created_by
FROM files WHERE id = ? AND deleted_at IS NULL;

-- name: SoftDeleteFile :execrows
UPDATE files SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL;

-- name: ListFiles :many
SELECT id, owner, name, content_type, size, sha256, storage_key, created_at, created_by
FROM files
WHERE deleted_at IS NULL
  AND (sqlc.arg(owner) = '' OR owner = sqlc.arg(owner))
  AND (sqlc.arg(search) = '' OR name LIKE CONCAT('%', sqlc.arg(search), '%'))
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountFiles :one
SELECT COUNT(*) FROM files
WHERE deleted_at IS NULL
  AND (sqlc.arg(owner) = '' OR owner = sqlc.arg(owner))
  AND (sqlc.arg(search) = '' OR name LIKE CONCAT('%', sqlc.arg(search), '%'));

-- name: PurgeCandidates :many
SELECT id, storage_key FROM files WHERE deleted_at < ? LIMIT 200;

-- name: DeleteFileRow :exec
DELETE FROM files WHERE id = ?;
