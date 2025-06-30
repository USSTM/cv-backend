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

-- name: ValidateSignupCode :one
SELECT sc.id, sc.email, sc.role_name, sc.scope, sc.scope_id, sc.expires_at
FROM signup_codes sc
WHERE sc.code = $1 AND sc.used_at IS NULL
  AND (sc.expires_at IS NULL OR sc.expires_at > NOW());

-- name: MarkSignupCodeUsed :exec
UPDATE signup_codes SET used_at = NOW() WHERE id = $1;