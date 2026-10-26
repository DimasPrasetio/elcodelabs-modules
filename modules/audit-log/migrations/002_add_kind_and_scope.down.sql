-- Back to one company: other companies' records would otherwise become company 1's.
DELETE FROM audit_log WHERE company_id <> 1;
ALTER TABLE audit_log
  DROP KEY audit_log_company,
  DROP COLUMN branch_id,
  DROP COLUMN company_id,
  DROP COLUMN kind;
