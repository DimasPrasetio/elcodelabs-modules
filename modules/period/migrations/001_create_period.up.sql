-- Closing the books: per company, the last date that is closed for every area ("*") or for one
-- area ("sales", "payroll", …). A posting dated on or before the later of the two is refused.
CREATE TABLE period_locks (
  company_id BIGINT NOT NULL,
  area VARCHAR(50) NOT NULL,
  locked_through DATE NOT NULL,
  updated_by BIGINT NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (company_id, area)
);

-- One row per company: its fiscal year, and the row every posting holds shared and every lock
-- change holds exclusive — so a lock never moves under a posting in flight.
CREATE TABLE period_settings (
  company_id BIGINT NOT NULL,
  fiscal_year_start_month TINYINT NOT NULL DEFAULT 1,
  updated_at DATETIME(6) NULL,
  PRIMARY KEY (company_id)
);
