-- name: GetAllItems :many
SELECT id, name, description, type, stock from items;

-- name: GetAllUsers :many
SELECT id, email from users;