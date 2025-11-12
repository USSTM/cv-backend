-- name: ListTimeSlots :many
SELECT * FROM time_slots
ORDER BY start_time;
