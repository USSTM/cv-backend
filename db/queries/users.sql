-- name: GetAllUsers :many
SELECT id, email from users;

-- name: CreateSignUpCode :one
INSERT INTO signup_codes (id, code, email, role_name, scope, scope_id, created_at, used_at, expires_at, created_by)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, NOW(), NULL, NOW() + INTERVAL '7 days', $6)
    RETURNING *;