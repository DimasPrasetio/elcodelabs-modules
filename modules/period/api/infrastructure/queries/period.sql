-- name: HoldShared :exec
-- Makes sure the company's settings row exists and, inside a transaction, holds it shared until
-- the end: a duplicate INSERT IGNORE takes a shared lock on the existing row.
INSERT IGNORE INTO period_settings (company_id, fiscal_year_start_month) VALUES (?, 1);

-- name: HoldExclusive :exec
INSERT INTO period_settings (company_id, fiscal_year_start_month) VALUES (?, 1)
ON DUPLICATE KEY UPDATE company_id = company_id;

-- name: LocksOfArea :many
SELECT area, locked_through FROM period_locks
WHERE company_id = sqlc.arg(company_id) AND area IN ('*', sqlc.arg(area));

-- name: LocksOfAreaShared :many
SELECT area, locked_through FROM period_locks
WHERE company_id = sqlc.arg(company_id) AND area IN ('*', sqlc.arg(area)) FOR SHARE;

-- name: Locks :many
SELECT area, locked_through, updated_by, updated_at FROM period_locks WHERE company_id = ? ORDER BY area;

-- name: PutLock :exec
INSERT INTO period_locks (company_id, area, locked_through, updated_by, updated_at) VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE locked_through = VALUES(locked_through), updated_by = VALUES(updated_by), updated_at = VALUES(updated_at);

-- name: DeleteLock :exec
DELETE FROM period_locks WHERE company_id = ? AND area = ?;

-- name: FiscalStart :one
SELECT fiscal_year_start_month FROM period_settings WHERE company_id = ?;

-- name: SetFiscalStart :exec
INSERT INTO period_settings (company_id, fiscal_year_start_month, updated_at) VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE fiscal_year_start_month = VALUES(fiscal_year_start_month), updated_at = VALUES(updated_at);
