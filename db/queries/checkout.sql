-- name: GetCartItemsForCheckout :many
SELECT c.group_id, c.user_id, c.item_id, c.quantity,
       i.type, i.stock, i.name
FROM cart c
JOIN items i ON c.item_id = i.id
WHERE c.group_id = $1 AND c.user_id = $2
FOR UPDATE OF i;

-- name: DecrementStockForLowItem :exec
UPDATE items
SET stock = stock - $2
WHERE id = $1
  AND type = 'low'
  AND stock >= $2;

-- name: RecordItemTaking :one
INSERT INTO item_takings (user_id, group_id, item_id, quantity)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, group_id, item_id, quantity, taken_at;
