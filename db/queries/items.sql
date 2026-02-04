-- name: GetAllItems :many
SELECT id, name, description, type, stock, urls from items ORDER BY name ASC LIMIT $1 OFFSET $2;

-- name: CreateItem :one
INSERT INTO items (name, description, type, stock, urls)
VALUES ($1, $2, $3, $4, sqlc.narg('urls'))
RETURNING id, name, description, type, stock, urls;

-- name: GetItemsByType :many
SELECT id, name, description, type, stock, urls FROM items WHERE type = $1 ORDER BY name ASC LIMIT $2 OFFSET $3;

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

-- name: GetItemByName :one
SELECT id, name, description, type, stock, urls
FROM items WHERE name = $1;

-- name: CountAllItems :one
SELECT COUNT(*) as count FROM items;

-- name: CountItemsByType :one
SELECT COUNT(*) as count FROM items WHERE type = $1;

-- name: SearchItems :many
WITH ranked_items AS (
    -- get rankings (each row turned to rank, from vector/query relationship)
    SELECT *,
    CASE
      WHEN sqlc.narg('query')::TEXT IS NOT NULL THEN
        ts_rank(
          to_tsvector('english', name || ' ' || COALESCE(description, '')),
          plainto_tsquery('english', sqlc.narg('query'))
        )
        -- 0 if null query
        ELSE 0.0
    END as rank
  FROM items
  WHERE (sqlc.narg('query')::TEXT IS NULL OR
  -- if nulls, no ranking shenanigans
    to_tsvector('english', name || ' ' || COALESCE(description, '')) @@ plainto_tsquery('english', sqlc.narg('query')))
    AND (sqlc.narg('item_type')::item_type IS NULL OR type = sqlc.narg('item_type'))
    AND (sqlc.narg('in_stock')::BOOLEAN IS NULL OR (stock > 0) = sqlc.narg('in_stock'))
)
SELECT id, name, description, type, stock, urls, rank
FROM ranked_items
ORDER BY
-- if query null then alphabetical, else sort by rank
  CASE WHEN sqlc.narg('query')::TEXT IS NOT NULL THEN rank END DESC NULLS LAST,
  name ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountSearchItems :one
SELECT COUNT(*) as count
FROM items
WHERE (sqlc.narg('query')::TEXT IS NULL OR
  to_tsvector('english', name || ' ' || COALESCE(description, '')) @@ plainto_tsquery('english', sqlc.narg('query')))
  AND (sqlc.narg('item_type')::item_type IS NULL OR type = sqlc.narg('item_type'))
  AND (sqlc.narg('in_stock')::BOOLEAN IS NULL OR (stock > 0) = sqlc.narg('in_stock'));
