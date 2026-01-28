-- name: GetGroupByID :one
SELECT id, name, description FROM groups WHERE id = $1;

-- name: GetAllGroups :many
SELECT id, name, description FROM groups ORDER BY name;

-- name: CreateGroup :one
INSERT INTO groups (name, description) VALUES ($1, $2)
RETURNING id, name, description;

-- name: UpdateGroup :one
UPDATE groups SET name = $2, description = $3 WHERE id = $1
RETURNING id, name, description;

-- name: DeleteGroup :exec
DELETE FROM groups WHERE id = $1;

-- name: GetGroupByName :one
SELECT id, name, description
FROM groups WHERE name = $1;