DELETE FROM user_roles WHERE company_id <> 1 OR branch_id <> 0;
ALTER TABLE user_roles
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (user_id, role_id),
  DROP COLUMN branch_id,
  DROP COLUMN company_id;
