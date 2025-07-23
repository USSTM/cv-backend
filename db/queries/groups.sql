-- name: GetGroupByID :one
SELECT id, name, description FROM groups WHERE id = $1;