-- A role is held in one company: across all of it (branch_id 0) or in one branch.
ALTER TABLE user_roles
  ADD COLUMN company_id BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN branch_id BIGINT NOT NULL DEFAULT 0,
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (user_id, role_id, company_id, branch_id);
