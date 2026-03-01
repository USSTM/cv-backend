-- name: GetNotificationEntityTypeByName :one
SELECT * FROM notification_entity_types
WHERE name = $1 LIMIT 1;

-- name: CreateNotificationObject :one
INSERT INTO notification_objects (
    entity_type_id, entity_id
) VALUES (
    $1, $2
)
RETURNING *;

-- name: CreateNotificationChange :one
INSERT INTO notification_changes (
    notification_object_id, actor_id
) VALUES (
    $1, $2
)
RETURNING *;

-- name: CreateNotification :one
INSERT INTO notifications (
    notification_object_id, notifier_id, is_read
) VALUES (
    $1, $2, false
)
RETURNING *;

-- name: GetUserNotifications :many
SELECT 
    n.id AS notification_id,
    n.is_read,
    n.created_at AS notification_created_at,
    no.id AS notification_object_id,
    no.entity_id,
    net.name AS entity_type_name,
    nc.actor_id,
    u.email AS actor_email
FROM notifications n
JOIN notification_objects no ON n.notification_object_id = no.id
JOIN notification_entity_types net ON no.entity_type_id = net.id
JOIN notification_changes nc ON no.id = nc.notification_object_id
JOIN users u ON nc.actor_id = u.id
WHERE n.notifier_id = $1
ORDER BY n.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUserNotifications :one
SELECT COUNT(*) FROM notifications
WHERE notifier_id = $1 AND is_read = false;

-- name: CountAllUserNotifications :one
SELECT COUNT(*) FROM notifications
WHERE notifier_id = $1;

-- name: MarkNotificationAsRead :one
UPDATE notifications
SET is_read = true
WHERE id = $1 AND notifier_id = $2
RETURNING *;

-- name: MarkAllNotificationsAsRead :exec
UPDATE notifications
SET is_read = true
WHERE notifier_id = $1 AND is_read = false;
