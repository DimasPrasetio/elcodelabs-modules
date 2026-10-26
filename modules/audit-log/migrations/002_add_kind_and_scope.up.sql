-- What kind of record (a change, a read or an export of personal data, a security event) and
-- the company/branch it happened in. Existing rows: changes in company 1.
ALTER TABLE audit_log
  ADD COLUMN kind VARCHAR(10) NOT NULL DEFAULT 'change',
  ADD COLUMN company_id BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN branch_id BIGINT NOT NULL DEFAULT 0,
  ADD KEY audit_log_company (company_id, occurred_at);
