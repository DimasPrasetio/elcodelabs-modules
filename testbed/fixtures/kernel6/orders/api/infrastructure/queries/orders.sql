-- name: CreateOrder :execlastid
INSERT INTO orders (company_id, sku, qty, created_at) VALUES (?, ?, ?, ?);

-- name: CountOrders :one
SELECT COUNT(*) FROM orders WHERE company_id = ?;
