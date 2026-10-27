-- name: ListSuppliers :many
SELECT * FROM supp_suppliers
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

-- name: CountSuppliers :one
SELECT COUNT(*) FROM supp_suppliers
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

-- name: SearchActiveSuppliers :many
SELECT * FROM supp_suppliers
WHERE company_id = sqlc.arg(company_id) AND is_active = TRUE
  AND (code LIKE CONCAT(sqlc.arg(search), '%')
    OR name LIKE CONCAT('%', sqlc.arg(search), '%')
    OR legal_name LIKE CONCAT('%', sqlc.arg(search), '%')
    OR tax_id LIKE CONCAT(sqlc.arg(search), '%'))
ORDER BY name, id
LIMIT ?;

-- name: GetSupplier :one
SELECT * FROM supp_suppliers WHERE company_id = ? AND id = ?;

-- name: GetSupplierShared :one
-- Held shared until the transaction ends: a posting that checked the supplier keeps a
-- deactivation or hold waiting, and the other way round.
SELECT * FROM supp_suppliers WHERE company_id = ? AND id = ? FOR SHARE;

-- name: GetSupplierForUpdate :one
SELECT * FROM supp_suppliers WHERE company_id = ? AND id = ? FOR UPDATE;

-- name: SuppliersWithTaxID :many
SELECT id, code, name, is_active FROM supp_suppliers
WHERE company_id = sqlc.arg(company_id) AND tax_id = sqlc.arg(tax_id) AND id <> sqlc.arg(except_id)
ORDER BY id
LIMIT 10;

-- name: CodeTaken :one
SELECT COUNT(*) FROM supp_suppliers WHERE company_id = ? AND code = ?;

-- name: InsertSupplier :execlastid
INSERT INTO supp_suppliers (company_id, code, name, legal_name, party_type, is_foreign, country, tax_id_kind, tax_id,
  is_pkp, email, phone, payment_days, notes, is_active, created_by, created_at, updated_by, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?, ?, ?);

-- name: UpdateSupplier :exec
UPDATE supp_suppliers SET code = ?, name = ?, legal_name = ?, party_type = ?, is_foreign = ?, country = ?, tax_id_kind = ?, tax_id = ?,
  is_pkp = ?, email = ?, phone = ?, payment_days = ?, notes = ?, updated_by = ?, updated_at = ?
WHERE company_id = ? AND id = ?;

-- name: SetSupplierActive :exec
UPDATE supp_suppliers SET is_active = ?, updated_by = ?, updated_at = ? WHERE company_id = ? AND id = ?;

-- name: SetSupplierHold :exec
UPDATE supp_suppliers SET on_hold = ?, hold_reason = ?, updated_by = ?, updated_at = ? WHERE company_id = ? AND id = ?;

-- name: ListAddresses :many
SELECT * FROM supp_addresses WHERE company_id = ? AND supplier_id = ? ORDER BY is_active DESC, id;

-- name: GetAddress :one
SELECT * FROM supp_addresses WHERE company_id = ? AND supplier_id = ? AND id = ?;

-- name: InsertAddress :execlastid
INSERT INTO supp_addresses (company_id, supplier_id, label, line, city, province, postal_code, country, nitku,
  for_billing, for_shipping, is_default_billing, is_default_shipping, is_active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?);

-- name: UpdateAddress :exec
UPDATE supp_addresses SET label = ?, line = ?, city = ?, province = ?, postal_code = ?, country = ?, nitku = ?,
  for_billing = ?, for_shipping = ?, is_default_billing = ?, is_default_shipping = ?, updated_at = ?
WHERE company_id = ? AND supplier_id = ? AND id = ?;

-- name: SetAddressActive :exec
UPDATE supp_addresses SET is_active = sqlc.arg(is_active), is_default_billing = is_default_billing AND sqlc.arg(is_active),
  is_default_shipping = is_default_shipping AND sqlc.arg(is_active), updated_at = sqlc.arg(updated_at)
WHERE company_id = sqlc.arg(company_id) AND supplier_id = sqlc.arg(supplier_id) AND id = sqlc.arg(id);

-- name: ClearDefaultBilling :exec
UPDATE supp_addresses SET is_default_billing = FALSE WHERE company_id = ? AND supplier_id = ? AND id <> ?;

-- name: ClearDefaultShipping :exec
UPDATE supp_addresses SET is_default_shipping = FALSE WHERE company_id = ? AND supplier_id = ? AND id <> ?;

-- name: ListBankAccounts :many
SELECT * FROM supp_bank_accounts WHERE company_id = ? AND supplier_id = ? ORDER BY is_active DESC, is_default DESC, id;

-- name: GetBankAccount :one
SELECT * FROM supp_bank_accounts WHERE company_id = ? AND supplier_id = ? AND id = ?;

-- name: InsertBankAccount :execlastid
INSERT INTO supp_bank_accounts (company_id, supplier_id, bank_name, bank_code, account_number, account_holder, is_default, is_active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?);

-- name: UpdateBankAccount :exec
UPDATE supp_bank_accounts SET bank_name = ?, bank_code = ?, account_number = ?, account_holder = ?, is_default = ?, updated_at = ?
WHERE company_id = ? AND supplier_id = ? AND id = ?;

-- name: SetBankAccountActive :exec
UPDATE supp_bank_accounts SET is_active = sqlc.arg(is_active), is_default = is_default AND sqlc.arg(is_active), updated_at = sqlc.arg(updated_at)
WHERE company_id = sqlc.arg(company_id) AND supplier_id = sqlc.arg(supplier_id) AND id = sqlc.arg(id);

-- name: ClearDefaultBankAccount :exec
UPDATE supp_bank_accounts SET is_default = FALSE WHERE company_id = ? AND supplier_id = ? AND id <> ?;

-- name: ListContacts :many
SELECT * FROM supp_contacts WHERE company_id = ? AND supplier_id = ? ORDER BY is_primary DESC, name, id;

-- name: GetContact :one
SELECT * FROM supp_contacts WHERE company_id = ? AND supplier_id = ? AND id = ?;

-- name: InsertContact :execlastid
INSERT INTO supp_contacts (company_id, supplier_id, name, title, email, phone, is_primary, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateContact :exec
UPDATE supp_contacts SET name = ?, title = ?, email = ?, phone = ?, is_primary = ?, updated_at = ?
WHERE company_id = ? AND supplier_id = ? AND id = ?;

-- name: DeleteContact :execrows
DELETE FROM supp_contacts WHERE company_id = ? AND supplier_id = ? AND id = ?;

-- name: ClearPrimaryContact :exec
UPDATE supp_contacts SET is_primary = FALSE WHERE company_id = ? AND supplier_id = ? AND id <> ?;

-- name: GetSettings :one
SELECT code_prefix, last_number FROM supp_settings WHERE company_id = ?;

-- name: EnsureSettings :exec
INSERT IGNORE INTO supp_settings (company_id, code_prefix, last_number, updated_at) VALUES (?, 'PMS-', 0, ?);

-- name: LockSettings :one
SELECT code_prefix, last_number FROM supp_settings WHERE company_id = ? FOR UPDATE;

-- name: SetLastNumber :exec
UPDATE supp_settings SET last_number = ? WHERE company_id = ?;

-- name: SetPrefix :exec
UPDATE supp_settings SET code_prefix = ?, updated_at = ? WHERE company_id = ?;
