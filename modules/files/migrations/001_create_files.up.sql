-- Files modules store through this one (a logo, an attachment). The bytes live on disk under
-- DATA_DIR/files; this row is what is known about them. Deleting sets deleted_at; the bytes go
-- 30 days later.
CREATE TABLE files (
  id BIGINT NOT NULL AUTO_INCREMENT,
  owner VARCHAR(40) NOT NULL,
  name VARCHAR(255) NOT NULL,
  content_type VARCHAR(100) NOT NULL,
  size BIGINT NOT NULL,
  sha256 CHAR(64) NOT NULL,
  storage_key VARCHAR(100) NOT NULL,
  created_at TIMESTAMP(6) NOT NULL,
  created_by BIGINT NULL,
  deleted_at TIMESTAMP(6) NULL,
  PRIMARY KEY (id),
  KEY files_owner (owner),
  KEY files_created (created_at),
  KEY files_deleted (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
