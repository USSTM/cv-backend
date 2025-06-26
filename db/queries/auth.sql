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