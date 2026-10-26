-- name: GetSetting :one
SELECT name, value, updated_at, updated_by FROM settings WHERE company_id = ? AND name = ?;

-- name: PutSetting :exec
INSERT INTO settings (company_id, name, value, updated_at, updated_by) VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE value = VALUES(value), updated_at = VALUES(updated_at), updated_by = VALUES(updated_by);

-- name: GetSettingForUpdate :one
SELECT value FROM settings WHERE company_id = ? AND name = ? FOR UPDATE;

-- name: EnsureSetting :exec
INSERT INTO settings (company_id, name, value, updated_at) VALUES (?, ?, '{}', ?)
ON DUPLICATE KEY UPDATE name = name;
