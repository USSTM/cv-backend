-- this function creates a new borrowing record for a user borrowing an item
-- name: BorrowItem :one
INSERT INTO borrowings (
    user_id, group_id, item_id, quantity,
    due_date, before_condition, before_condition_url
)
SELECT $1, $2, i.id, $4, $5, $6, $7
FROM items i
WHERE i.id = $3
  AND i.type IN ('medium', 'high')
  AND i.stock >= $4
RETURNING id, user_id, group_id, item_id, quantity,
    borrowed_at, due_date, returned_at,
    before_condition, before_condition_url,
    after_condition, after_condition_url;

-- this function records the return of a borrowed item, updating the after condition and return timestamp (basically closing the borrowing record)
-- it only works if the item is currently borrowed (i.e., has no return timestamp yet)
-- the request is identified by the item_id
-- name: ReturnItem :one
UPDATE borrowings
SET returned_at = NOW(),
    after_condition = $2,
    after_condition_url = $3
WHERE item_id = $1 AND returned_at IS NULL
RETURNING id, user_id, group_id, item_id, quantity,
    borrowed_at, due_date, returned_at,
    before_condition, before_condition_url,
    after_condition, after_condition_url;

-- this function checks if an item is currently borrowed (i.e., not available) by looking for active borrowings without a return timestamp and returns true if the item is available
-- name: CheckBorrowingItemStatus :one
SELECT NOT EXISTS (
    SELECT 1 FROM borrowings
    WHERE item_id = $1 AND returned_at IS NULL
) AS is_available;

-- this function gets an active borrowing by item_id and user_id, used to validate ownership before return
-- name: GetActiveBorrowingByItemAndUser :one
SELECT id, user_id, group_id, item_id, quantity,
       borrowed_at, due_date, returned_at,
       before_condition, before_condition_url,
       after_condition, after_condition_url
FROM borrowings
WHERE item_id = $1 AND user_id = $2 AND returned_at IS NULL
FOR UPDATE;

-- name: GetBorrowedItemHistoryByUserId :many
SELECT id, user_id, group_id, item_id, quantity,
       borrowed_at, due_date, returned_at,
       before_condition, before_condition_url,
       after_condition, after_condition_url
FROM borrowings
WHERE user_id = $1;

-- name: GetActiveBorrowedItemsByUserId :many
SELECT id, user_id, group_id, item_id, quantity,
       borrowed_at, due_date, returned_at,
       before_condition, before_condition_url,
       after_condition, after_condition_url
FROM borrowings
WHERE user_id = $1 AND returned_at IS NULL;

-- name: GetReturnedItemsByUserId :many
SELECT id, user_id, group_id, item_id, quantity,
       borrowed_at, due_date, returned_at,
       before_condition, before_condition_url,
       after_condition, after_condition_url
FROM borrowings
WHERE user_id = $1 AND returned_at IS NOT NULL;

-- name: GetAllActiveBorrowedItems :many
SELECT id, user_id, group_id, item_id, quantity,
       borrowed_at, due_date, returned_at,
       before_condition, before_condition_url,
       after_condition, after_condition_url
FROM borrowings
WHERE returned_at IS NULL;

-- name: GetAllReturnedItems :many
SELECT id, user_id, group_id, item_id, quantity,
       borrowed_at, due_date, returned_at,
       before_condition, before_condition_url,
       after_condition, after_condition_url
FROM borrowings
WHERE returned_at IS NOT NULL;

-- name: GetActiveBorrowedItemsToBeReturnedByDate :many
SELECT id, user_id, group_id, item_id, quantity,
       borrowed_at, due_date, returned_at,
       before_condition, before_condition_url,
       after_condition, after_condition_url
FROM borrowings
WHERE returned_at IS NULL AND due_date <= $1;