-- "Forgot password" requests. Only a hash of each token is stored: a leaked table cannot reset
-- anyone's password. A token works once, for 30 minutes, and a newer request voids older ones.
-- It is also bound to the address it was sent to and to the password at that time: a later
-- change of either voids it.
CREATE TABLE auth_password_resets (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_id BIGINT NOT NULL,
  token_hash CHAR(64) NOT NULL,
  email VARCHAR(254) NOT NULL,
  created_at TIMESTAMP(6) NOT NULL,
  expires_at TIMESTAMP(6) NOT NULL,
  used_at TIMESTAMP(6) NULL,
  ip VARCHAR(45) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY auth_password_resets_token (token_hash),
  KEY auth_password_resets_user (user_id),
  KEY auth_password_resets_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
