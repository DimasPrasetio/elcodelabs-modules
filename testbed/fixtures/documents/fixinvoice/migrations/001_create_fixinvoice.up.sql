CREATE TABLE fix_invoices (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  company_id BIGINT NOT NULL,
  branch_id BIGINT NOT NULL DEFAULT 0,
  doc_date DATE NOT NULL,
  number VARCHAR(100) NULL,
  discount DECIMAL(19,2) NOT NULL,
  total DECIMAL(19,2) NOT NULL,
  status VARCHAR(10) NOT NULL DEFAULT 'draft',
  posted_at DATETIME(6) NULL,
  posted_by BIGINT NULL,
  cancelled_at DATETIME(6) NULL,
  cancelled_by BIGINT NULL,
  cancel_reason VARCHAR(500) NULL,
  amended_from BIGINT NULL,
  revision INT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY fix_invoices_number (company_id, number),
  UNIQUE KEY fix_invoices_amended (company_id, amended_from)
);

CREATE TABLE fix_invoice_lines (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  company_id BIGINT NOT NULL,
  invoice_id BIGINT NOT NULL,
  sku VARCHAR(50) NOT NULL,
  qty DECIMAL(19,4) NOT NULL,
  price DECIMAL(19,2) NOT NULL,
  amount DECIMAL(19,2) NOT NULL,
  discount_share DECIMAL(19,2) NOT NULL,
  KEY fix_invoice_lines_invoice (company_id, invoice_id)
);

CREATE TABLE fix_stock (
  company_id BIGINT NOT NULL,
  sku VARCHAR(50) NOT NULL,
  qty DECIMAL(19,4) NOT NULL,
  PRIMARY KEY (company_id, sku)
);
