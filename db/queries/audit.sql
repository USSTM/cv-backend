-- name: GetTakingHistoryByUserId :many
SELECT t.id, t.user_id, t.group_id, t.item_id, t.quantity, t.taken_at,
       i.name, i.description, i.type
FROM item_takings t
JOIN items i ON t.item_id = i.id
WHERE t.user_id = $1
ORDER BY t.taken_at DESC
LIMIT $2 OFFSET $3;

-- name: GetTakingHistoryByUserIdWithGroupFilter :many
SELECT t.id, t.user_id, t.group_id, t.item_id, t.quantity, t.taken_at,
       i.name, i.description, i.type
FROM item_takings t
JOIN items i ON t.item_id = i.id
WHERE t.user_id = $1 AND t.group_id = $2
ORDER BY t.taken_at DESC
LIMIT $3 OFFSET $4;

-- name: GetTakingHistoryByItemId :many
SELECT t.id, t.user_id, t.group_id, t.item_id, t.quantity, t.taken_at,
       u.email as user_email
FROM item_takings t
JOIN users u ON t.user_id = u.id
WHERE t.item_id = $1
ORDER BY t.taken_at DESC
LIMIT $2 OFFSET $3;

-- name: GetTakingStats :one
SELECT
    COUNT(*) as total_takings,
    COALESCE(SUM(quantity), 0) as total_quantity,
    COUNT(DISTINCT user_id) as unique_users,
    MIN(taken_at) as first_taking,
    MAX(taken_at) as last_taking
FROM item_takings
WHERE item_id = $1
  AND taken_at >= $2
  AND taken_at <= $3;

-- name: CountTakingHistoryByUserId :one
SELECT COUNT(*) as count FROM item_takings WHERE user_id = $1;

-- name: CountTakingHistoryByUserIdWithGroupFilter :one
SELECT COUNT(*) as count FROM item_takings WHERE user_id = $1 AND group_id = $2;

-- name: CountTakingHistoryByItemId :one
SELECT COUNT(*) as count FROM item_takings WHERE item_id = $1;
