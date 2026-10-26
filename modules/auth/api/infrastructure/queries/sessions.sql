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
