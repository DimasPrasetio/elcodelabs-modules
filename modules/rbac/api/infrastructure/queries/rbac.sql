-- name: ListRoles :many
SELECT r.id, r.name, r.description, r.is_system, r.created_at, r.updated_at,
       (SELECT COUNT(*) FROM user_roles ur WHERE ur.role_id = r.id) AS member_count
FROM roles r
ORDER BY r.is_system DESC, r.name;

-- name: GetRole :one
SELECT r.id, r.name, r.description, r.is_system, r.created_at, r.updated_at,
       (SELECT COUNT(*) FROM user_roles ur WHERE ur.role_id = r.id) AS member_count
FROM roles r
WHERE r.id = ?;

-- name: GetRoleByName :one
SELECT id FROM roles WHERE name = ?;

-- name: ListRolePermissions :many
SELECT role_id, permission FROM role_permissions ORDER BY role_id, permission;

-- name: ListPermissionsOfRole :many
SELECT permission FROM role_permissions WHERE role_id = ? ORDER BY permission;

-- name: CreateRole :execlastid
INSERT INTO roles (name, description) VALUES (?, ?);

-- name: UpdateRole :exec
UPDATE roles SET name = ?, description = ? WHERE id = ?;

-- name: DeleteRole :execrows
DELETE FROM roles WHERE id = ?;

-- name: DeleteRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = ?;

-- name: AddRolePermission :exec
INSERT INTO role_permissions (role_id, permission) VALUES (?, ?);

-- name: ListMembers :many
SELECT user_id FROM user_roles WHERE role_id = ? ORDER BY created_at, user_id;

-- name: CountMembers :one
SELECT COUNT(*) FROM user_roles WHERE role_id = ?;

-- name: AddMember :exec
INSERT IGNORE INTO user_roles (user_id, role_id) VALUES (?, ?);

-- name: RemoveMember :execrows
DELETE FROM user_roles WHERE user_id = ? AND role_id = ?;

-- name: PermissionsOfUser :many
SELECT DISTINCT rp.permission
FROM user_roles ur
JOIN role_permissions rp ON rp.role_id = ur.role_id
WHERE ur.user_id = ?;
