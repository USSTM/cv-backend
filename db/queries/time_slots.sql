-- name: ListTimeSlots :many
SELECT * FROM time_slots
ORDER BY start_time;

-- name: GetTimeSlotByID :one
SELECT * FROM time_slots
WHERE id = $1;

-- name: GetTimeSlotByStartTime :one
SELECT * FROM time_slots
WHERE start_time = $1;
