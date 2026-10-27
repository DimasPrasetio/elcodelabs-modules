-- A company's own format for a document type; without a row the type uses the format the
-- declaring module gave.
CREATE TABLE num_formats (
  company_id BIGINT NOT NULL,
  doc_type VARCHAR(100) NOT NULL,
  pattern VARCHAR(100) NOT NULL,
  reset VARCHAR(10) NOT NULL,
  per_branch BOOLEAN NOT NULL,
  updated_by BIGINT NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (company_id, doc_type)
);

-- One counter per series: company, document type, branch (0: not per branch), and reset period
-- ("" never, "2026", "2026-10", "2026-10-27"). Taken under a row lock in the caller's
-- transaction, so a rollback gives the number back.
CREATE TABLE num_counters (
  company_id BIGINT NOT NULL,
  doc_type VARCHAR(100) NOT NULL,
  branch_id BIGINT NOT NULL DEFAULT 0,
  period_key VARCHAR(10) NOT NULL,
  last_number BIGINT NOT NULL,
  PRIMARY KEY (company_id, doc_type, branch_id, period_key)
);

-- Every number given, per company and type: Next never prints one twice, whatever happens to
-- codes (a branch renamed, a code reused) or formats. Rolled back with the posting.
CREATE TABLE num_issued (
  company_id BIGINT NOT NULL,
  doc_type VARCHAR(100) NOT NULL,
  number VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL, -- "inv-1" is not "INV-1"
  PRIMARY KEY (company_id, doc_type, number)
);
