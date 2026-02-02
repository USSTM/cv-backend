-- name: CreateBooking :one
INSERT INTO booking (
    id, requester_id, manager_id, item_id, group_id, availability_id,
    pick_up_date, pick_up_location, return_date, return_location, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetBookingByID :one
SELECT
    b.*,
    requester.email as requester_email,
    manager.email as manager_email,
    i.name as item_name,
    i.type as item_type,
    ua.date as availability_date,
    g.name as group_name,
    ts.start_time,
    ts.end_time
FROM booking b
JOIN users requester ON b.requester_id = requester.id
LEFT JOIN users manager ON b.manager_id = manager.id
JOIN items i ON b.item_id = i.id
JOIN groups g ON b.group_id = g.id
JOIN user_availability ua ON b.availability_id = ua.id
JOIN time_slots ts ON ua.time_slot_id = ts.id
WHERE b.id = $1;

-- name: GetBookingByIDForUpdate :one
SELECT * FROM booking WHERE id = $1 FOR UPDATE;

-- name: ListBookings :many
SELECT
    b.*,
    requester.email as requester_email,
    manager.email as manager_email,
    i.name as item_name,
    ua.date as availability_date,
    g.name as group_name
FROM booking b
JOIN users requester ON b.requester_id = requester.id
LEFT JOIN users manager ON b.manager_id = manager.id
JOIN items i ON b.item_id = i.id
JOIN groups g ON b.group_id = g.id
JOIN user_availability ua ON b.availability_id = ua.id
WHERE (sqlc.narg('status')::request_status IS NULL OR b.status = sqlc.narg('status'))
  AND (sqlc.narg('group_id')::UUID IS NULL OR b.group_id = sqlc.narg('group_id'))
  AND (sqlc.narg('from_date')::DATE IS NULL OR ua.date >= sqlc.narg('from_date'))
  AND (sqlc.narg('to_date')::DATE IS NULL OR ua.date <= sqlc.narg('to_date'))
ORDER BY ua.date, b.pick_up_date
LIMIT $1 OFFSET $2;

-- name: ListBookingsByUser :many
SELECT
    b.*,
    manager.email as manager_email,
    i.name as item_name,
    ua.date as availability_date,
    ts.start_time,
    ts.end_time
FROM booking b
LEFT JOIN users manager ON b.manager_id = manager.id
JOIN items i ON b.item_id = i.id
JOIN user_availability ua ON b.availability_id = ua.id
JOIN time_slots ts ON ua.time_slot_id = ts.id
WHERE b.requester_id = $1
  AND (sqlc.narg('status')::request_status IS NULL OR b.status = sqlc.narg('status'))
ORDER BY ua.date DESC
LIMIT $2 OFFSET $3;

-- name: ListPendingConfirmation :many
SELECT
    b.*,
    requester.email as requester_email,
    i.name as item_name,
    ua.date as availability_date,
    g.name as group_name,
    ts.start_time
FROM booking b
JOIN users requester ON b.requester_id = requester.id
JOIN items i ON b.item_id = i.id
JOIN groups g ON b.group_id = g.id
JOIN user_availability ua ON b.availability_id = ua.id
JOIN time_slots ts ON ua.time_slot_id = ts.id
WHERE b.status = 'pending_confirmation'
  AND (sqlc.narg('group_id')::UUID IS NULL OR b.group_id = sqlc.narg('group_id'))
ORDER BY ua.date, ts.start_time;

-- name: ConfirmBooking :one
UPDATE booking
SET status = 'confirmed',
    confirmed_at = NOW(),
    confirmed_by = $2
WHERE id = $1
RETURNING *;

-- name: CancelBooking :one
UPDATE booking
SET status = 'cancelled'
WHERE id = $1
RETURNING *;

-- name: GetExpiredBookings :many
SELECT id FROM booking
WHERE status = 'pending_confirmation'
  AND created_at < NOW() - INTERVAL '48 hours';

-- name: CountBookings :one
SELECT COUNT(*) as count
FROM booking b
JOIN user_availability ua ON b.availability_id = ua.id
WHERE (sqlc.narg('status')::request_status IS NULL OR b.status = sqlc.narg('status'))
  AND (sqlc.narg('group_id')::UUID IS NULL OR b.group_id = sqlc.narg('group_id'))
  AND (sqlc.narg('from_date')::DATE IS NULL OR ua.date >= sqlc.narg('from_date'))
  AND (sqlc.narg('to_date')::DATE IS NULL OR ua.date <= sqlc.narg('to_date'));

-- name: CountBookingsByUser :one
SELECT COUNT(*) as count
FROM booking b
WHERE b.requester_id = $1
  AND (sqlc.narg('status')::request_status IS NULL OR b.status = sqlc.narg('status'));
