-- Companies (legal entities), their branches, and where each user is placed. Company 1 is the
-- one every module's existing data belongs to (company_id DEFAULT 1).
CREATE TABLE org_companies (
  id BIGINT NOT NULL AUTO_INCREMENT,
  code VARCHAR(20) NOT NULL,
  name VARCHAR(120) NOT NULL,
  timezone VARCHAR(64) NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY org_companies_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE org_branches (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL,
  code VARCHAR(20) NOT NULL,
  name VARCHAR(120) NOT NULL,
  timezone VARCHAR(64) NULL,
  nitku CHAR(22) NULL,
  address VARCHAR(255) NOT NULL DEFAULT '',
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY org_branches_code (company_id, code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE org_placements (
  user_id BIGINT NOT NULL,
  company_id BIGINT NOT NULL,
  branch_id BIGINT NOT NULL DEFAULT 0,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  created_at DATETIME(6) NOT NULL,
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, company_id, branch_id),
  KEY org_placements_company (company_id, branch_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Its name and time zone come from APP_NAME and APP_TIMEZONE on the first start.
INSERT INTO org_companies (id, code, name, timezone, is_active, created_at, updated_at)
VALUES (1, 'PUSAT', 'Perusahaan utama', 'Asia/Jakarta', TRUE, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6));
