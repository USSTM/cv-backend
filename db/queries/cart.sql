-- name: AddToCart :one
INSERT INTO cart (group_id, user_id, item_id, quantity)
VALUES ($1, $2, $3, $4)
ON CONFLICT (group_id, user_id, item_id)
DO UPDATE SET quantity = cart.quantity + EXCLUDED.quantity
RETURNING group_id, user_id, item_id, quantity, created_at;

-- name: RemoveFromCart :exec
DELETE FROM cart
WHERE group_id = $1 AND user_id = $2 AND item_id = $3;

-- name: UpdateCartItemQuantity :one
UPDATE cart
SET quantity = $4
WHERE group_id = $1 AND user_id = $2 AND item_id = $3
RETURNING group_id, user_id, item_id, quantity, created_at;

-- name: GetCartByUser :many
SELECT c.group_id, c.user_id, c.item_id, c.quantity, c.created_at,
       i.name, i.description, i.type, i.stock, i.urls
FROM cart c
JOIN items i ON c.item_id = i.id
WHERE c.group_id = $1 AND c.user_id = $2
ORDER BY c.created_at DESC;

-- name: ClearCart :exec
DELETE FROM cart
WHERE group_id = $1 AND user_id = $2;

-- name: GetCartItemCount :one
SELECT COUNT(*) as item_count, COALESCE(SUM(quantity), 0) as total_quantity
FROM cart
WHERE group_id = $1 AND user_id = $2;
