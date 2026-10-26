DELETE FROM settings WHERE company_id <> 1;
ALTER TABLE settings
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (name),
  DROP COLUMN company_id;
