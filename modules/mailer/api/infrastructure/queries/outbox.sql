-- name: Enqueue :execlastid
INSERT INTO mail_outbox (to_address, to_name, subject, text_body, html_body, status, next_attempt_at, created_at)
VALUES (?, ?, ?, ?, ?, 'pending', ?, ?);

-- name: DueIDs :many
SELECT id FROM mail_outbox
WHERE status = 'pending' AND next_attempt_at <= ?
ORDER BY next_attempt_at, id
LIMIT ?
FOR UPDATE SKIP LOCKED;

-- name: MarkSending :exec
UPDATE mail_outbox SET status = 'sending', locked_until = ? WHERE id = ?;

-- name: GetMessage :one
SELECT id, to_address, to_name, subject, text_body, html_body, status, attempts, next_attempt_at, last_error, created_at, sent_at
FROM mail_outbox WHERE id = ?;

-- name: MarkSent :exec
UPDATE mail_outbox SET status = 'sent', attempts = attempts + 1, sent_at = ?, locked_until = NULL, last_error = '', text_body = '', html_body = ''
WHERE id = ?;

-- name: MarkFailedAttempt :exec
UPDATE mail_outbox SET status = ?, attempts = ?, next_attempt_at = ?, locked_until = NULL, last_error = ? WHERE id = ?;

-- name: ReleaseStuck :execrows
UPDATE mail_outbox SET status = 'pending', locked_until = NULL
WHERE status = 'sending' AND locked_until < ?;

-- name: Requeue :execrows
UPDATE mail_outbox SET status = 'pending', attempts = 0, next_attempt_at = ?, last_error = ''
WHERE id = ? AND status = 'failed';

-- name: Prune :execrows
DELETE FROM mail_outbox
WHERE (status = 'sent' AND created_at < sqlc.arg(sent_before)) OR (status = 'failed' AND created_at < sqlc.arg(failed_before))
LIMIT 1000;

-- name: ListMessages :many
SELECT id, to_address, to_name, subject, status, attempts, next_attempt_at, last_error, created_at, sent_at
FROM mail_outbox
WHERE (sqlc.arg(status) = '' OR status = sqlc.arg(status))
  AND (sqlc.arg(search) = '' OR to_address LIKE CONCAT('%', sqlc.arg(search), '%') OR subject LIKE CONCAT('%', sqlc.arg(search), '%'))
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: CountMessages :one
SELECT COUNT(*) FROM mail_outbox
WHERE (sqlc.arg(status) = '' OR status = sqlc.arg(status))
  AND (sqlc.arg(search) = '' OR to_address LIKE CONCAT('%', sqlc.arg(search), '%') OR subject LIKE CONCAT('%', sqlc.arg(search), '%'));
