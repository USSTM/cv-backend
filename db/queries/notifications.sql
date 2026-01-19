-- name: CreateNotification :one
INSERT INTO notifications (
    user_id, type, title, message, priority, expires_at,
    related_booking_id, related_request_id, related_item_id, related_user_id, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
) RETURNING *;

-- name: GetNotification :one
SELECT * FROM notifications
WHERE id = $1;

-- name: GetUserNotifications :many
SELECT * FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUnreadUserNotifications :many
SELECT * FROM notifications
WHERE user_id = $1 AND read_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetNotificationsByType :many
SELECT * FROM notifications
WHERE user_id = $1 AND type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetNotificationsByPriority :many
SELECT * FROM notifications
WHERE user_id = $1 AND priority = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: MarkNotificationAsRead :one
UPDATE notifications
SET read_at = NOW()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: MarkAllNotificationsAsRead :exec
UPDATE notifications
SET read_at = NOW()
WHERE user_id = $1 AND read_at IS NULL;

-- name: MarkNotificationsByTypeAsRead :exec
UPDATE notifications
SET read_at = NOW()
WHERE user_id = $1 AND type = $2 AND read_at IS NULL;

-- name: DeleteNotification :exec
DELETE FROM notifications
WHERE id = $1 AND user_id = $2;

-- name: DeleteReadNotifications :exec
DELETE FROM notifications
WHERE user_id = $1 AND read_at IS NOT NULL;

-- name: DeleteExpiredNotifications :exec
DELETE FROM notifications
WHERE expires_at IS NOT NULL AND expires_at < NOW();

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notifications
WHERE user_id = $1 AND read_at IS NULL;

-- name: CountNotificationsByType :one
SELECT COUNT(*) FROM notifications
WHERE user_id = $1 AND type = $2 AND read_at IS NULL;

-- name: GetNotificationStats :one
SELECT
    COUNT(*) as total_notifications,
    COUNT(*) FILTER (WHERE read_at IS NULL) as unread_count,
    COUNT(*) FILTER (WHERE priority = 'high') as high_priority_count,
    COUNT(*) FILTER (WHERE priority = 'high' AND read_at IS NULL) as unread_high_priority_count
FROM notifications
WHERE user_id = $1;

-- name: BulkCreateNotifications :copyfrom
INSERT INTO notifications (
    user_id, type, title, message, priority, expires_at,
    related_booking_id, related_request_id, related_item_id, related_user_id, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
);

-- name: GetRecentNotifications :many
SELECT * FROM notifications
WHERE user_id = $1 AND created_at >= $2
ORDER BY created_at DESC;

-- name: UpdateNotificationMetadata :one
UPDATE notifications
SET metadata = $3
WHERE id = $1 AND user_id = $2
RETURNING *;
