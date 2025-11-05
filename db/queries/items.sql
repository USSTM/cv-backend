-- name: GetAllItems :many
SELECT id, name, description, type, stock, urls from items;

-- name: CreateItem :one
INSERT INTO items (name, description, type, stock, urls)
VALUES ($1, $2, $3, $4, sqlc.narg('urls'))
RETURNING id, name, description, type, stock, urls;

-- name: GetItemsByType :many
SELECT id, name, description, type, stock, urls FROM items WHERE type = $1;

-- name: GetItemByID :one
SELECT id, name, description, type, stock, urls FROM items WHERE id = $1;

-- name: GetItemByIDForUpdate :one
SELECT id, name, description, type, stock, urls FROM items WHERE id = $1 FOR UPDATE;

-- name: UpdateItem :one
UPDATE items
SET name = $2, description = $3, type = $4, stock = $5, urls = $6
WHERE id = $1
RETURNING id, name, description, type, stock, urls;

-- name: DeleteItem :exec
DELETE FROM items WHERE id = $1;

-- name: PatchItem :one
UPDATE items
SET name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    type = COALESCE(sqlc.narg('type'), type),
    stock = COALESCE(sqlc.narg('stock'), stock),
    urls = COALESCE(sqlc.narg('urls'), urls)
WHERE id = sqlc.arg('id')
RETURNING id, name, description, type, stock, urls;

-- name: DecrementItemStock :exec
UPDATE items
SET stock = stock - $2
WHERE id = $1 AND stock >= $2;

-- name: IncrementItemStock :exec
UPDATE items
SET stock = stock + $2
WHERE id = $1;