-- name: AddStock :exec
INSERT INTO ledger_stock (company_id, sku, qty) VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE qty = qty + VALUES(qty);

-- name: TakeStock :execrows
UPDATE ledger_stock SET qty = qty - sqlc.arg(take) WHERE company_id = sqlc.arg(company_id) AND sku = sqlc.arg(sku) AND qty >= sqlc.arg(take);

-- name: AddNote :exec
INSERT INTO ledger_notes (company_id, body) VALUES (?, ?);
