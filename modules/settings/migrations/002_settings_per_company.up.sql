-- Each company keeps its own settings (kernel 6 scope). Existing settings: company 1's.
ALTER TABLE settings
  ADD COLUMN company_id BIGINT NOT NULL DEFAULT 1 FIRST,
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (company_id, name);
