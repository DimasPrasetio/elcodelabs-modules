-- Who did what, when — written once, never changed. Old rows are removed after the retention
-- period (AUDIT_RETENTION_DAYS) by a daily job.
CREATE TABLE audit_log (
  id BIGINT NOT NULL AUTO_INCREMENT,
  occurred_at TIMESTAMP(6) NOT NULL,
  actor_id BIGINT NULL,
  module VARCHAR(40) NOT NULL,
  action VARCHAR(80) NOT NULL,
  target VARCHAR(100) NOT NULL DEFAULT '',
  summary VARCHAR(300) NOT NULL DEFAULT '',
  -- JSON null when there is no detail: a NULL column cannot be read into the generated type.
  data JSON NOT NULL,
  ip VARCHAR(45) NOT NULL DEFAULT '',
  request_id VARCHAR(64) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY audit_log_occurred (occurred_at),
  KEY audit_log_module_action (module, action),
  KEY audit_log_target (target),
  KEY audit_log_actor (actor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
