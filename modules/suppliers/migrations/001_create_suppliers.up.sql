-- Suppliers of a company. Deactivated, never deleted: documents point at them.
CREATE TABLE supp_suppliers (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  code VARCHAR(30) NOT NULL,
  name VARCHAR(150) NOT NULL,
  legal_name VARCHAR(200) NOT NULL DEFAULT '',      -- the name on tax invoices; '' = name
  party_type VARCHAR(12) NOT NULL,                  -- company | individual | government
  is_foreign BOOLEAN NOT NULL DEFAULT FALSE,
  country CHAR(2) NOT NULL DEFAULT 'ID',            -- ISO 3166-1 alpha-2
  tax_id_kind VARCHAR(10) NOT NULL DEFAULT 'none',  -- npwp | nik | passport | none
  tax_id VARCHAR(30) NOT NULL DEFAULT '',           -- NPWP16 / NIK digits, or a passport number
  is_pkp BOOLEAN NOT NULL DEFAULT FALSE,
  email VARCHAR(190) NOT NULL DEFAULT '',
  phone VARCHAR(30) NOT NULL DEFAULT '',
  payment_days INT NULL,                            -- NULL: cash on delivery
  on_hold BOOLEAN NOT NULL DEFAULT FALSE,
  hold_reason VARCHAR(255) NOT NULL DEFAULT '',
  notes TEXT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_by BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_by BIGINT NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY supp_suppliers_code (company_id, code),
  KEY supp_suppliers_tax (company_id, tax_id),
  KEY supp_suppliers_name (company_id, is_active, name)
);

-- Billing and shipping addresses. One NPWP has many places of business (NITKU), each with its
-- own address. Deactivated, never deleted: a document's snapshot names the address it used.
CREATE TABLE supp_addresses (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  supplier_id BIGINT NOT NULL,
  label VARCHAR(60) NOT NULL,
  line VARCHAR(255) NOT NULL,
  city VARCHAR(100) NOT NULL DEFAULT '',
  province VARCHAR(100) NOT NULL DEFAULT '',
  postal_code VARCHAR(10) NOT NULL DEFAULT '',
  country CHAR(2) NOT NULL DEFAULT 'ID',
  nitku CHAR(22) NULL,
  for_billing BOOLEAN NOT NULL DEFAULT TRUE,
  for_shipping BOOLEAN NOT NULL DEFAULT TRUE,
  is_default_billing BOOLEAN NOT NULL DEFAULT FALSE,
  is_default_shipping BOOLEAN NOT NULL DEFAULT FALSE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY supp_addresses_supplier (company_id, supplier_id)
);

-- Where the supplier is paid. Deactivated, never deleted: a payment names the account it used.
-- Personal financial data for an individual supplier (UU PDP): masked, whole only with an
-- access record.
CREATE TABLE supp_bank_accounts (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  supplier_id BIGINT NOT NULL,
  bank_name VARCHAR(100) NOT NULL,
  bank_code VARCHAR(11) NOT NULL DEFAULT '',     -- 3-digit Indonesian bank code, or SWIFT/BIC
  account_number VARCHAR(34) NOT NULL,           -- 34: the longest IBAN
  account_holder VARCHAR(150) NOT NULL,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY supp_bank_accounts_supplier (company_id, supplier_id)
);

-- People at the supplier. Nothing points at them: removed when no longer needed.
CREATE TABLE supp_contacts (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  supplier_id BIGINT NOT NULL,
  name VARCHAR(150) NOT NULL,
  title VARCHAR(100) NOT NULL DEFAULT '',
  email VARCHAR(190) NOT NULL DEFAULT '',
  phone VARCHAR(30) NOT NULL DEFAULT '',
  is_primary BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY supp_contacts_supplier (company_id, supplier_id)
);

-- The prefix and counter of generated codes, per company. Taken under the row's lock; gaps are
-- fine for master data.
CREATE TABLE supp_settings (
  company_id BIGINT NOT NULL,
  code_prefix VARCHAR(10) NOT NULL DEFAULT 'PMS-',
  last_number BIGINT NOT NULL DEFAULT 0,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (company_id)
);
