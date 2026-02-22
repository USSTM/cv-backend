-- name: GetGroupByID :one
SELECT id, name, description, logo_s3_key, logo_thumbnail_s3_key FROM groups WHERE id = $1;

-- name: GetAllGroups :many
SELECT id, name, description, logo_s3_key, logo_thumbnail_s3_key FROM groups ORDER BY name;

-- name: CreateGroup :one
INSERT INTO groups (name, description) VALUES ($1, $2)
RETURNING id, name, description, logo_s3_key, logo_thumbnail_s3_key;

-- name: UpdateGroup :one
UPDATE groups SET name = $2, description = $3 WHERE id = $1
RETURNING id, name, description, logo_s3_key, logo_thumbnail_s3_key;

-- name: DeleteGroup :exec
DELETE FROM groups WHERE id = $1;

-- name: GetGroupByName :one
SELECT id, name, description, logo_s3_key, logo_thumbnail_s3_key
FROM groups WHERE name = $1;

-- name: UpdateGroupLogo :one
UPDATE groups SET logo_s3_key = $2, logo_thumbnail_s3_key = $3 WHERE id = $1
RETURNING id, name, description, logo_s3_key, logo_thumbnail_s3_key;
