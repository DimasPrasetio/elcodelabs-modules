-- When the password last changed: an auth provider ends every session opened before it.
-- Microseconds, so a sign-in in the same second as the change is still told apart.
ALTER TABLE users ADD COLUMN password_changed_at TIMESTAMP(6) NULL;
