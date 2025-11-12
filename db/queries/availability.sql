-- name: CreateAvailability :one
INSERT INTO user_availability (id, user_id, time_slot_id, date)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetAvailabilityByID :one
SELECT
  ua.*,
  u.email as user_email,
  ts.start_time,
  ts.end_time
FROM user_availability ua
JOIN users u ON ua.user_id = u.id
JOIN time_slots ts ON ua.time_slot_id = ts.id
WHERE ua.id = $1;

-- name: ListAvailability :many
SELECT
  ua.*,
  u.email as user_email,
  ts.start_time,
  ts.end_time
FROM user_availability ua
JOIN users u ON ua.user_id = u.id
JOIN time_slots ts ON ua.time_slot_id = ts.id
WHERE (sqlc.narg('filter_date')::DATE IS NULL OR ua.date = sqlc.narg('filter_date'))
  AND (sqlc.narg('filter_user_id')::UUID IS NULL OR ua.user_id = sqlc.narg('filter_user_id'))
ORDER BY ua.date, ts.start_time;

-- name: GetAvailabilityByDate :many
-- Get all approvers available on a specific date
SELECT
  ua.*,
  u.email as user_email,
  ts.start_time,
  ts.end_time
FROM user_availability ua
JOIN users u ON ua.user_id = u.id
JOIN time_slots ts ON ua.time_slot_id = ts.id
WHERE ua.date = $1
ORDER BY ts.start_time, u.email;

-- name: GetUserAvailability :many
-- Get a specific user's availability schedule
SELECT
  ua.*,
  ts.start_time,
  ts.end_time
FROM user_availability ua
JOIN time_slots ts ON ua.time_slot_id = ts.id
WHERE ua.user_id = $1
  AND (sqlc.narg('from_date')::DATE IS NULL OR ua.date >= sqlc.narg('from_date'))
  AND (sqlc.narg('to_date')::DATE IS NULL OR ua.date <= sqlc.narg('to_date'))
ORDER BY ua.date, ts.start_time;

-- name: CheckAvailabilityConflict :one
-- Check if user already has availability for this slot/date
SELECT EXISTS(
  SELECT 1 FROM user_availability
  WHERE user_id = $1
    AND time_slot_id = $2
    AND date = $3
) AS has_conflict;

-- name: DeleteAvailability :exec
DELETE FROM user_availability WHERE id = $1;

-- name: CheckAvailabilityInUse :one
-- Check if availability is referenced by active bookings
SELECT EXISTS(
  SELECT 1 FROM booking
  WHERE availability_id = $1
    AND status IN ('pending', 'approved')
) AS in_use;

-- name: GetAvailableApproversForSlot :many
-- Find all approvers available for a specific date/time slot
SELECT DISTINCT u.id, u.email
FROM user_availability ua
JOIN users u ON ua.user_id = u.id
WHERE ua.date = $1
  AND ua.time_slot_id = $2;

-- name: GetAvailabilityCountByUser :one
-- Get count of availability entries for a user in a date range
SELECT COUNT(*) as count
FROM user_availability
WHERE user_id = $1
  AND date >= $2
  AND date <= $3;
