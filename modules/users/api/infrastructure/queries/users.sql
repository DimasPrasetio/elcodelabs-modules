-- name: ListUsers :many
SELECT id, name, email, is_active, created_at, updated_at
FROM users
WHERE sqlc.arg(search) = ''
   OR name LIKE CONCAT('%', sqlc.arg(search), '%')
   OR email LIKE CONCAT('%', sqlc.arg(search), '%')
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountUsers :one
SELECT COUNT(*)
FROM users
WHERE sqlc.arg(search) = ''
   OR name LIKE CONCAT('%', sqlc.arg(search), '%')
   OR email LIKE CONCAT('%', sqlc.arg(search), '%');

-- name: CountAllUsers :one
SELECT COUNT(*) FROM users;

-- name: GetUser :one
SELECT id, name, email, is_active, created_at, updated_at, password_changed_at
FROM users
WHERE id = ?;

-- name: GetUserByEmail :one
SELECT id, name, email, is_active, created_at, updated_at
FROM users
WHERE email = ?;

-- name: GetCredentialsByEmail :one
SELECT id, password_hash, is_active
FROM users
WHERE email = ?;

-- name: CreateUser :execlastid
INSERT INTO users (name, email, password_hash, is_active)
VALUES (?, ?, ?, ?);

-- name: UpdateUser :execrows
UPDATE users SET name = ?, email = ?, is_active = ? WHERE id = ?;

-- name: UpdateUserName :execrows
UPDATE users SET name = ? WHERE id = ?;

-- name: UpdatePassword :exec
UPDATE users SET password_hash = ?, password_changed_at = ? WHERE id = ?;
