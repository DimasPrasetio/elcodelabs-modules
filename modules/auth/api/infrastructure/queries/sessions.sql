-- name: CreateSession :exec
INSERT INTO auth_sessions (id, user_id, created_at, expires_at, ip, user_agent)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetSession :one
SELECT id, user_id, created_at, expires_at, revoked_at
FROM auth_sessions
WHERE id = ?;

-- name: RevokeSession :exec
UPDATE auth_sessions SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :exec
DELETE FROM auth_sessions WHERE expires_at < ? LIMIT 500;

-- name: CreateReset :exec
INSERT INTO auth_password_resets (user_id, token_hash, email, created_at, expires_at, ip)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetResetByHash :one
SELECT id, user_id, email, created_at, expires_at, used_at FROM auth_password_resets WHERE token_hash = ?;

-- name: UseReset :execrows
UPDATE auth_password_resets SET used_at = ? WHERE id = ? AND used_at IS NULL;

-- name: ReleaseReset :exec
UPDATE auth_password_resets SET used_at = NULL WHERE id = ? AND used_at = ?;

-- name: VoidUserResets :exec
UPDATE auth_password_resets SET used_at = ? WHERE user_id = ? AND used_at IS NULL;

-- name: DeleteOldResets :exec
DELETE FROM auth_password_resets WHERE expires_at < ? LIMIT 500;
