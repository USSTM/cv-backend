-- this function creates a new request in the requests table for a user requesting an item
-- name: RequestItem :one
INSERT INTO requests (
    user_id, group_id, item_id, quantity, status
)
SELECT $1, $2, i.id, $4, 'pending'
FROM items i
WHERE i.id = $3 AND i.type = 'high'
RETURNING id, user_id, group_id, item_id, quantity,
    status, reviewed_at, reviewed_by;

-- this function updates the status of a request (approve or deny) and records who reviewed it and when
-- name: ReviewRequest :one
UPDATE requests r
SET status = $2,
    reviewed_by = $3,
    reviewed_at = NOW()
FROM items i
WHERE r.id = $1
  AND r.status = 'pending'
  AND r.item_id = i.id
  AND (
    $2 = 'denied'::request_status
    OR ($2 = 'approved'::request_status AND i.stock >= r.quantity)
  )
RETURNING r.id, r.user_id, r.group_id, r.item_id, r.quantity,
    r.status, r.reviewed_at, r.reviewed_by;

-- name: GetApprovedRequestForUserAndItem :one
SELECT * FROM requests
WHERE user_id = $1
  AND item_id = $2
  AND status = 'approved'
  AND fulfilled_at IS NULL
ORDER BY reviewed_at DESC
LIMIT 1;

-- name: MarkRequestAsFulfilled :exec
UPDATE requests
SET fulfilled_at = NOW()
WHERE id = $1;

-- name: GetPendingRequests :many
SELECT * FROM requests
WHERE status = 'pending'
ORDER BY requested_at ASC;

-- name: GetAllRequests :many
SELECT * FROM requests
ORDER BY requested_at DESC;

-- name: GetRequestsByUserId :many
SELECT * FROM requests
WHERE user_id = $1
ORDER BY requested_at DESC;

-- name: GetRequestById :one
SELECT * FROM requests
WHERE id = $1;

-- name: GetRequestByIdForUpdate :one
SELECT * FROM requests
WHERE id = $1
FOR UPDATE;