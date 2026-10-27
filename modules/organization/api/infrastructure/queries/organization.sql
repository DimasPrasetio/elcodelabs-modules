-- name: ListCompanies :many
SELECT id, code, name, timezone, is_active, created_at, updated_at FROM org_companies ORDER BY id;

-- name: ListActiveCompanies :many
SELECT id, code, name, timezone, is_active, created_at, updated_at FROM org_companies WHERE is_active = TRUE ORDER BY id;

-- name: GetCompany :one
SELECT id, code, name, timezone, is_active, created_at, updated_at FROM org_companies WHERE id = ?;

-- name: CreateCompany :execlastid
INSERT INTO org_companies (code, name, timezone, is_active, created_at, updated_at) VALUES (?, ?, ?, TRUE, ?, ?);

-- name: UpdateCompany :execrows
UPDATE org_companies SET code = ?, name = ?, timezone = ?, updated_at = ? WHERE id = ?;

-- name: SetCompanyActive :execrows
UPDATE org_companies SET is_active = ?, updated_at = ? WHERE id = ?;

-- name: ListBranches :many
SELECT id, company_id, code, name, timezone, nitku, address, is_active, created_at, updated_at
FROM org_branches WHERE company_id = ? ORDER BY code;

-- name: GetBranch :one
SELECT id, company_id, code, name, timezone, nitku, address, is_active, created_at, updated_at
FROM org_branches WHERE id = ?;

-- name: CreateBranch :execlastid
INSERT INTO org_branches (company_id, code, name, timezone, nitku, address, is_active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, TRUE, ?, ?);

-- name: UpdateBranch :execrows
UPDATE org_branches SET code = ?, name = ?, timezone = ?, nitku = ?, address = ?, updated_at = ?
WHERE id = ? AND company_id = ?;

-- name: SetBranchActive :execrows
UPDATE org_branches SET is_active = ?, updated_at = ? WHERE id = ? AND company_id = ?;

-- name: PlacementsOfUser :many
-- The user's placements in active companies (a branch placement's branch: BranchesOfUser).
SELECT p.company_id, p.branch_id, p.is_default, c.code AS company_code, c.name AS company_name, c.timezone AS company_timezone
FROM org_placements p
JOIN org_companies c ON c.id = p.company_id AND c.is_active = TRUE
WHERE p.user_id = ?
ORDER BY p.company_id, p.branch_id;

-- name: BranchesOfUser :many
-- The active branches the user is placed in.
SELECT b.id, b.company_id, b.code, b.name, b.timezone
FROM org_placements p
JOIN org_branches b ON b.id = p.branch_id AND b.company_id = p.company_id AND b.is_active = TRUE
WHERE p.user_id = ? AND p.branch_id <> 0;

-- name: PlacesOfUser :many
SELECT user_id, company_id, branch_id, is_default, created_at, created_by
FROM org_placements WHERE user_id = ? ORDER BY company_id, branch_id;

-- name: ListPlacements :many
SELECT user_id, company_id, branch_id, is_default, created_at, created_by
FROM org_placements WHERE company_id = ? ORDER BY user_id, branch_id;

-- name: AddPlacement :execrows
INSERT IGNORE INTO org_placements (user_id, company_id, branch_id, is_default, created_at, created_by) VALUES (?, ?, ?, ?, ?, ?);

-- name: RemovePlacement :execrows
DELETE FROM org_placements WHERE user_id = ? AND company_id = ? AND branch_id = ?;

-- name: ClearDefault :exec
UPDATE org_placements SET is_default = FALSE WHERE user_id = ?;

-- name: SetDefault :execrows
UPDATE org_placements SET is_default = TRUE WHERE user_id = ? AND company_id = ? AND branch_id = ?;

-- name: PlacedUserIDs :many
SELECT DISTINCT user_id FROM org_placements WHERE company_id = ? ORDER BY user_id;

-- name: CountUserInCompany :one
SELECT COUNT(*) FROM org_placements WHERE user_id = ? AND company_id = ?;

-- name: CountPlacementsOfUser :one
SELECT COUNT(*) FROM org_placements WHERE user_id = ?;

-- name: CountPlacements :one
SELECT COUNT(*) FROM org_placements;

-- name: UsersWithAWorkplace :many
-- Users placed somewhere they can work: across an active company, or in an active branch of one.
SELECT p.user_id FROM org_placements p
JOIN org_companies c ON c.id = p.company_id AND c.is_active = TRUE
WHERE p.branch_id = 0
UNION
SELECT p.user_id FROM org_placements p
JOIN org_companies c ON c.id = p.company_id AND c.is_active = TRUE
JOIN org_branches b ON b.id = p.branch_id AND b.company_id = p.company_id AND b.is_active = TRUE;
