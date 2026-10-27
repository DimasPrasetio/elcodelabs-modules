-- name: ListCustomers :many
SELECT * FROM cust_customers
WHERE company_id = sqlc.arg(company_id)
  AND (sqlc.arg(status) = ''
    OR (sqlc.arg(status) = 'active' AND is_active = TRUE)
    OR (sqlc.arg(status) = 'on_hold' AND is_active = TRUE AND on_hold = TRUE)
    OR (sqlc.arg(status) = 'inactive' AND is_active = FALSE))
  AND (sqlc.arg(search) = ''
    OR code LIKE CONCAT('%', sqlc.arg(search), '%')
    OR name LIKE CONCAT('%', sqlc.arg(search), '%')
    OR legal_name LIKE CONCAT('%', sqlc.arg(search), '%')
    OR tax_id LIKE CONCAT(sqlc.arg(search), '%'))
ORDER BY name, id
LIMIT ? OFFSET ?;

-- name: CountCustomers :one
SELECT COUNT(*) FROM cust_customers
WHERE company_id = sqlc.arg(company_id)
  AND (sqlc.arg(status) = ''
    OR (sqlc.arg(status) = 'active' AND is_active = TRUE)
    OR (sqlc.arg(status) = 'on_hold' AND is_active = TRUE AND on_hold = TRUE)
    OR (sqlc.arg(status) = 'inactive' AND is_active = FALSE))
  AND (sqlc.arg(search) = ''
    OR code LIKE CONCAT('%', sqlc.arg(search), '%')
    OR name LIKE CONCAT('%', sqlc.arg(search), '%')
    OR legal_name LIKE CONCAT('%', sqlc.arg(search), '%')
    OR tax_id LIKE CONCAT(sqlc.arg(search), '%'));

-- name: SearchActiveCustomers :many
SELECT * FROM cust_customers
WHERE company_id = sqlc.arg(company_id) AND is_active = TRUE
  AND (code LIKE CONCAT(sqlc.arg(search), '%')
    OR name LIKE CONCAT('%', sqlc.arg(search), '%')
    OR legal_name LIKE CONCAT('%', sqlc.arg(search), '%')
    OR tax_id LIKE CONCAT(sqlc.arg(search), '%'))
ORDER BY name, id
LIMIT ?;

-- name: GetCustomer :one
SELECT * FROM cust_customers WHERE company_id = ? AND id = ?;

-- name: GetCustomerShared :one
-- Held shared until the transaction ends: a posting that checked the customer keeps a
-- deactivation or hold waiting, and the other way round.
SELECT * FROM cust_customers WHERE company_id = ? AND id = ? FOR SHARE;

-- name: GetCustomerForUpdate :one
SELECT * FROM cust_customers WHERE company_id = ? AND id = ? FOR UPDATE;

-- name: CustomersWithTaxID :many
SELECT id, code, name, is_active FROM cust_customers
WHERE company_id = sqlc.arg(company_id) AND tax_id = sqlc.arg(tax_id) AND id <> sqlc.arg(except_id)
ORDER BY id
LIMIT 10;

-- name: CodeTaken :one
SELECT COUNT(*) FROM cust_customers WHERE company_id = ? AND code = ?;

-- name: InsertCustomer :execlastid
INSERT INTO cust_customers (company_id, code, name, legal_name, party_type, is_foreign, country, tax_id_kind, tax_id,
  is_pkp, vat_collector, email, phone, payment_days, credit_limit, notes, is_active, created_by, created_at, updated_by, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?, ?, ?);

-- name: UpdateCustomer :exec
UPDATE cust_customers SET code = ?, name = ?, legal_name = ?, party_type = ?, is_foreign = ?, country = ?, tax_id_kind = ?, tax_id = ?,
  is_pkp = ?, vat_collector = ?, email = ?, phone = ?, payment_days = ?, credit_limit = ?, notes = ?, updated_by = ?, updated_at = ?
WHERE company_id = ? AND id = ?;

-- name: SetCustomerActive :exec
UPDATE cust_customers SET is_active = ?, updated_by = ?, updated_at = ? WHERE company_id = ? AND id = ?;

-- name: SetCustomerHold :exec
UPDATE cust_customers SET on_hold = ?, hold_reason = ?, updated_by = ?, updated_at = ? WHERE company_id = ? AND id = ?;

-- name: ListAddresses :many
SELECT * FROM cust_addresses WHERE company_id = ? AND customer_id = ? ORDER BY is_active DESC, id;

-- name: GetAddress :one
SELECT * FROM cust_addresses WHERE company_id = ? AND customer_id = ? AND id = ?;

-- name: InsertAddress :execlastid
INSERT INTO cust_addresses (company_id, customer_id, label, line, city, province, postal_code, country, nitku,
  for_billing, for_shipping, is_default_billing, is_default_shipping, is_active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?);

-- name: UpdateAddress :exec
UPDATE cust_addresses SET label = ?, line = ?, city = ?, province = ?, postal_code = ?, country = ?, nitku = ?,
  for_billing = ?, for_shipping = ?, is_default_billing = ?, is_default_shipping = ?, updated_at = ?
WHERE company_id = ? AND customer_id = ? AND id = ?;

-- name: SetAddressActive :exec
UPDATE cust_addresses SET is_active = sqlc.arg(is_active), is_default_billing = is_default_billing AND sqlc.arg(is_active),
  is_default_shipping = is_default_shipping AND sqlc.arg(is_active), updated_at = sqlc.arg(updated_at)
WHERE company_id = sqlc.arg(company_id) AND customer_id = sqlc.arg(customer_id) AND id = sqlc.arg(id);

-- name: ClearDefaultBilling :exec
UPDATE cust_addresses SET is_default_billing = FALSE WHERE company_id = ? AND customer_id = ? AND id <> ?;

-- name: ClearDefaultShipping :exec
UPDATE cust_addresses SET is_default_shipping = FALSE WHERE company_id = ? AND customer_id = ? AND id <> ?;

-- name: ListContacts :many
SELECT * FROM cust_contacts WHERE company_id = ? AND customer_id = ? ORDER BY is_primary DESC, name, id;

-- name: GetContact :one
SELECT * FROM cust_contacts WHERE company_id = ? AND customer_id = ? AND id = ?;

-- name: InsertContact :execlastid
INSERT INTO cust_contacts (company_id, customer_id, name, title, email, phone, is_primary, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateContact :exec
UPDATE cust_contacts SET name = ?, title = ?, email = ?, phone = ?, is_primary = ?, updated_at = ?
WHERE company_id = ? AND customer_id = ? AND id = ?;

-- name: DeleteContact :execrows
DELETE FROM cust_contacts WHERE company_id = ? AND customer_id = ? AND id = ?;

-- name: ClearPrimaryContact :exec
UPDATE cust_contacts SET is_primary = FALSE WHERE company_id = ? AND customer_id = ? AND id <> ?;

-- name: GetSettings :one
SELECT code_prefix, last_number FROM cust_settings WHERE company_id = ?;

-- name: EnsureSettings :exec
INSERT IGNORE INTO cust_settings (company_id, code_prefix, last_number, updated_at) VALUES (?, 'PLG-', 0, ?);

-- name: LockSettings :one
SELECT code_prefix, last_number FROM cust_settings WHERE company_id = ? FOR UPDATE;

-- name: SetLastNumber :exec
UPDATE cust_settings SET last_number = ? WHERE company_id = ?;

-- name: SetPrefix :exec
UPDATE cust_settings SET code_prefix = ?, updated_at = ? WHERE company_id = ?;
