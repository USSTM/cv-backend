-- name: GetUserByEmail :one
SELECT id, email, password_hash, role FROM users WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, role) 
VALUES ($1, $2, $3) 
RETURNING id, email, role;

-- name: GetUserByID :one
SELECT id, email, role FROM users WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;