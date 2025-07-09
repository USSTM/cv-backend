-- name: GetAllItems :many
SELECT id, name, description, type, stock from items;

-- name: CreateItem :one
INSERT INTO items (name, description, type, stock)
VALUES ($1, $2, $3, $4)
RETURNING id, name, description, type, stock;

-- name: GetItemsByType :many
SELECT id, name, description, type, stock FROM items WHERE type = $1;

-- name: GetItemByID :one
SELECT id, name, description, type, stock FROM items WHERE id = $1;

-- name: UpdateItem :one
UPDATE items
SET name = $2, description = $3, type = $4, stock = $5
WHERE id = $1
RETURNING id, name, description, type, stock;

-- name: DeleteItem :exec
DELETE FROM items WHERE id = $1;