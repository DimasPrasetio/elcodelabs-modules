-- name: CreateInvoice :execlastid
INSERT INTO fix_invoices (company_id, branch_id, customer_id, doc_date, number, discount, total, amended_from, revision, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: AddLine :exec
INSERT INTO fix_invoice_lines (company_id, invoice_id, sku, qty, price, amount, discount_share) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetInvoice :one
SELECT * FROM fix_invoices WHERE id = ? AND company_id = ?;

-- name: GetInvoiceForUpdate :one
SELECT * FROM fix_invoices WHERE id = ? AND company_id = ? FOR UPDATE;

-- name: Lines :many
SELECT sku, qty, price, amount, discount_share FROM fix_invoice_lines WHERE company_id = ? AND invoice_id = ? ORDER BY id;

-- name: DeleteLines :exec
DELETE FROM fix_invoice_lines WHERE company_id = ? AND invoice_id = ?;

-- name: UpdateDraft :execrows
UPDATE fix_invoices SET doc_date = ?, discount = ?, total = ? WHERE id = ? AND company_id = ? AND status = 'draft';

-- name: MarkPosted :execrows
UPDATE fix_invoices SET status = 'posted', number = ?, posted_at = ?, posted_by = ?
WHERE id = ? AND company_id = ? AND status = 'draft';

-- name: SetCustomerSnapshot :exec
UPDATE fix_invoices SET customer_name = ?, customer_tax_id = ? WHERE id = ? AND company_id = ?;

-- name: MarkCancelled :execrows
UPDATE fix_invoices SET status = 'cancelled', cancelled_at = ?, cancelled_by = ?, cancel_reason = ?
WHERE id = ? AND company_id = ? AND status = 'posted';

-- name: DeleteDraft :execrows
DELETE FROM fix_invoices WHERE id = ? AND company_id = ? AND status = 'draft';

-- name: AddStock :exec
INSERT INTO fix_stock (company_id, sku, qty) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE qty = qty + VALUES(qty);

-- name: TakeStock :execrows
UPDATE fix_stock SET qty = qty - sqlc.arg(qty) WHERE company_id = sqlc.arg(company_id) AND sku = sqlc.arg(sku) AND qty >= sqlc.arg(qty);

-- name: ReturnStock :exec
UPDATE fix_stock SET qty = qty + sqlc.arg(qty) WHERE company_id = sqlc.arg(company_id) AND sku = sqlc.arg(sku);
