-- One row per sign-in. The session token names its row (jti), so a session can be ended
-- before it expires: on logout, or when the user's password changes.
CREATE TABLE auth_sessions (
  id CHAR(32) NOT NULL,
  user_id BIGINT NOT NULL,
  -- Microseconds: a session must compare exactly against users' password_changed_at.
  created_at TIMESTAMP(6) NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  revoked_at TIMESTAMP NULL,
  ip VARCHAR(45) NOT NULL DEFAULT '',
  user_agent VARCHAR(255) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY auth_sessions_user (user_id),
  KEY auth_sessions_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
