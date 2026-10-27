-- The customer an invoice is for (a plain ID — customers is another module), and what the
-- invoice copied of it when posted.
ALTER TABLE fix_invoices
  ADD COLUMN customer_id BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN customer_name VARCHAR(200) NOT NULL DEFAULT '',
  ADD COLUMN customer_tax_id VARCHAR(30) NOT NULL DEFAULT '';
