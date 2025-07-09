-- name: GetUserByEmail :one
SELECT id, email, password_hash FROM users WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash) 
VALUES ($1, $2) 
RETURNING id, email;

-- name: GetUserByID :one
SELECT id, email FROM users WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: GetUserPermissions :many
SELECT DISTINCT p.name, p.description, ur.scope, ur.scope_id
FROM permissions p
JOIN role_permissions rp ON p.name = rp.permission_name
JOIN user_roles ur ON rp.role_name = ur.role_name
WHERE ur.user_id = $1;

-- name: GetUserRoles :many
SELECT ur.role_name, r.description, ur.scope, ur.scope_id
FROM user_roles ur
JOIN roles r ON ur.role_name = r.name
WHERE ur.user_id = $1;

-- name: CheckUserPermission :one
SELECT COUNT(*) > 0 as has_permission
FROM permissions p
JOIN role_permissions rp ON p.name = rp.permission_name
JOIN user_roles ur ON rp.role_name = ur.role_name
WHERE ur.user_id = $1 AND p.name = $2
  AND (ur.scope = 'global' OR (ur.scope = 'group' AND ur.scope_id = $3));


-- name: CreateRole :exec
INSERT INTO roles (name, description) VALUES ($1, $2);

-- name: CreatePermission :exec
INSERT INTO permissions (name, description) VALUES ($1, $2);

-- name: CreateRolePermission :exec
INSERT INTO role_permissions (role_name, permission_name) VALUES ($1, $2);

-- name: CreateUserRole :exec
INSERT INTO user_roles (user_id, role_name, scope, scope_id) VALUES ($1, $2, $3, $4);

-- name: CreateGroup :one
INSERT INTO groups (name, description) VALUES ($1, $2) RETURNING id, name, description;

