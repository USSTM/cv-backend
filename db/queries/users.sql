-- name: GetAllUsers :many
SELECT id, email from users;

-- name: GetUsersByGroup :many
SELECT u.id, u.email, ur.role_name, ur.scope, ur.scope_id
FROM users u
JOIN user_roles ur on u.id = ur.user_id
WHERE ur.scope = 'group' AND ur.scope_id = $1;

-- name: GetUserGroupsByUserId :many
SELECT scope_id
FROM user_roles
WHERE user_id = $1 AND scope = 'group' AND scope_id IS NOT NULL;

-- name: IsUserMemberOfGroup :one
SELECT EXISTS(
  SELECT 1 FROM user_roles
  WHERE user_id = $1
    AND scope = 'group'
    AND scope_id = $2
) AS is_member;

-- name: GetUsersByIDs :many
SELECT id, email FROM users WHERE id = ANY(@ids::uuid[]);

-- name: CreateSignUpCode :one
INSERT INTO signup_codes (id, code, email, role_name, scope, scope_id, created_at, used_at, expires_at, created_by)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NOW(), NULL, NOW() + INTERVAL '7 days', $6)
    RETURNING *;