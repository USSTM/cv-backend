-- name: GetAllItems :many
SELECT id, name, description, type, stock, urls from items;

-- name: CreateItem :one
INSERT INTO items (name, description, type, stock, urls)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name, description, type, stock, urls;

-- name: GetItemsByType :many
SELECT id, name, description, type, stock, urls FROM items WHERE type = $1;

-- name: GetItemByID :one
SELECT id, name, description, type, stock, urls FROM items WHERE id = $1;

-- name: UpdateItem :one
UPDATE items
SET name = $2, description = $3, type = $4, stock = $5, urls = $6
WHERE id = $1
RETURNING id, name, description, type, stock, urls;

-- name: DeleteItem :exec
DELETE FROM items WHERE id = $1;

-- name: UpdateItemStock :exec
UPDATE items SET stock = $2 WHERE id = $1;