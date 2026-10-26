-- App-wide settings as named JSON documents ("company", …), each with who changed it last.
CREATE TABLE settings (
  name VARCHAR(100) NOT NULL,
  value JSON NOT NULL,
  updated_at TIMESTAMP(6) NOT NULL,
  updated_by BIGINT NULL,
  PRIMARY KEY (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
