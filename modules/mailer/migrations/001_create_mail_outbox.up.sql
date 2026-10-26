-- Every email the app sends, in the order it was asked for. A row is written in the request that
-- asks (fast, and survives a crash); a background job sends it and records the outcome.
CREATE TABLE mail_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT,
  to_address VARCHAR(254) NOT NULL,
  to_name VARCHAR(120) NOT NULL DEFAULT '',
  subject VARCHAR(200) NOT NULL,
  text_body MEDIUMTEXT NOT NULL,
  html_body MEDIUMTEXT NOT NULL,
  status VARCHAR(10) NOT NULL DEFAULT 'pending',
  attempts INT NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMP(6) NOT NULL,
  locked_until TIMESTAMP(6) NULL,
  last_error VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMP(6) NOT NULL,
  sent_at TIMESTAMP(6) NULL,
  PRIMARY KEY (id),
  KEY mail_outbox_due (status, next_attempt_at),
  KEY mail_outbox_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
