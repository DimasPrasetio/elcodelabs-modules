-- Back to one company: other companies' files would otherwise become company 1's. Their bytes
-- stay under DATA_DIR/files until removed by hand.
DELETE FROM files WHERE company_id <> 1;
ALTER TABLE files
  DROP KEY files_company,
  DROP COLUMN locked_at,
  DROP COLUMN company_id;
