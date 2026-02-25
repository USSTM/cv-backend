-- name: CreateItemImage :one
INSERT INTO item_images (id, item_id, original_s3_key, thumbnail_s3_key, display_order, is_primary, width, height, uploaded_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetItemImageByID :one
SELECT * FROM item_images WHERE id = $1;

-- name: ListItemImagesByItem :many
SELECT * FROM item_images WHERE item_id = $1 ORDER BY display_order ASC, created_at ASC;

-- name: UnsetPrimaryItemImages :exec
UPDATE item_images SET is_primary = FALSE WHERE item_id = $1;

-- name: SetItemImageAsPrimary :exec
UPDATE item_images SET is_primary = TRUE WHERE id = $1;

-- name: DeleteItemImage :exec
DELETE FROM item_images WHERE id = $1;
