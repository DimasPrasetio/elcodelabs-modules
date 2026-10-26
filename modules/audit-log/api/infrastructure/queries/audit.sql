-- name: InsertEntry :exec
INSERT INTO audit_log (occurred_at, actor_id, kind, company_id, branch_id, module, action, target, summary, data, ip, request_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListEntries :many
SELECT id, occurred_at, actor_id, kind, branch_id, module, action, target, summary, data, ip, request_id
FROM audit_log
WHERE company_id = sqlc.arg(company_id)
  AND (sqlc.arg(kind) = '' OR kind = sqlc.arg(kind))
  AND (sqlc.arg(module) = '' OR module = sqlc.arg(module))
  AND (sqlc.arg(actor) = 0 OR actor_id = sqlc.arg(actor))
  AND (sqlc.arg(target) = '' OR target = sqlc.arg(target))
  AND occurred_at >= sqlc.arg(from_at) AND occurred_at < sqlc.arg(to_at)
ORDER BY occurred_at DESC, id DESC
LIMIT ? OFFSET ?;

-- name: CountEntries :one
SELECT COUNT(*) FROM audit_log
WHERE company_id = sqlc.arg(company_id)
  AND (sqlc.arg(kind) = '' OR kind = sqlc.arg(kind))
  AND (sqlc.arg(module) = '' OR module = sqlc.arg(module))
  AND (sqlc.arg(actor) = 0 OR actor_id = sqlc.arg(actor))
  AND (sqlc.arg(target) = '' OR target = sqlc.arg(target))
  AND occurred_at >= sqlc.arg(from_at) AND occurred_at < sqlc.arg(to_at);

-- name: DistinctModules :many
SELECT DISTINCT module FROM audit_log WHERE company_id = ? ORDER BY module;

-- name: SystemDeleteOlderThan :execrows
-- all companies: the retention period is the app's.
DELETE FROM audit_log WHERE occurred_at < ? LIMIT 5000;
