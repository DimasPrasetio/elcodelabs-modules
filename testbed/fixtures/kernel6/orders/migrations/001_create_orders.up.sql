CREATE TABLE orders (
  id BIGINT NOT NULL AUTO_INCREMENT,
  company_id BIGINT NOT NULL DEFAULT 1,
  sku VARCHAR(40) NOT NULL,
  qty INT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY orders_company (company_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
