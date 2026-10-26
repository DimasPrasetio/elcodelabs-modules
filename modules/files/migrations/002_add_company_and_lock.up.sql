-- Files belong to a company (kernel 6 scope); a file attached to a posted document is locked:
-- it can no longer be deleted. Existing files: company 1, unlocked.
ALTER TABLE files
  ADD COLUMN company_id BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN locked_at DATETIME(6) NULL,
  ADD KEY files_company (company_id, id);
