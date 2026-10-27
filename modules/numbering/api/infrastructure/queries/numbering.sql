-- name: Formats :many
SELECT doc_type, pattern, reset, per_branch, updated_by, updated_at FROM num_formats WHERE company_id = ?;

-- name: FormatOf :one
SELECT pattern, reset, per_branch FROM num_formats WHERE company_id = ? AND doc_type = ?;

-- name: FormatOfShared :one
SELECT pattern, reset, per_branch FROM num_formats WHERE company_id = ? AND doc_type = ? FOR SHARE;

-- name: PutFormat :exec
INSERT INTO num_formats (company_id, doc_type, pattern, reset, per_branch, updated_by, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE pattern = VALUES(pattern), reset = VALUES(reset), per_branch = VALUES(per_branch),
  updated_by = VALUES(updated_by), updated_at = VALUES(updated_at);

-- name: DeleteFormat :exec
DELETE FROM num_formats WHERE company_id = ? AND doc_type = ?;

-- name: Bump :exec
-- Takes the next value of a series: creates it at 1, or adds 1 under the row's exclusive lock,
-- held until the caller's transaction ends.
INSERT INTO num_counters (company_id, doc_type, branch_id, period_key, last_number) VALUES (?, ?, ?, ?, 1)
ON DUPLICATE KEY UPDATE last_number = last_number + 1;

-- name: LastValueForUpdate :one
SELECT last_number FROM num_counters
WHERE company_id = ? AND doc_type = ? AND branch_id = ? AND period_key = ? FOR UPDATE;

-- name: EnsureSeries :exec
INSERT IGNORE INTO num_counters (company_id, doc_type, branch_id, period_key, last_number) VALUES (?, ?, ?, ?, 0);

-- name: LastValue :one
SELECT last_number FROM num_counters
WHERE company_id = ? AND doc_type = ? AND branch_id = ? AND period_key = ?;

-- name: SetLastValue :exec
INSERT INTO num_counters (company_id, doc_type, branch_id, period_key, last_number) VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE last_number = VALUES(last_number);

-- name: IssuedShared :one
-- Whether any number of the type was taken — read with a shared lock, so a posting taking the
-- first one waits for a format change, or the change for it.
SELECT COUNT(*) FROM num_counters WHERE company_id = ? AND doc_type = ? AND last_number > 0 FOR SHARE;

-- name: RecordIssued :execrows
INSERT IGNORE INTO num_issued (company_id, doc_type, number) VALUES (?, ?, ?);

-- name: IsIssued :one
SELECT COUNT(*) FROM num_issued WHERE company_id = ? AND doc_type = ? AND number = ?;
