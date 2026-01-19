# Notifications System

This document describes the notifications system implementation in the CV Backend.

## Overview

The notifications system provides a comprehensive solution for managing user notifications with the following features:

- **Persistent Storage**: Notifications are stored in PostgreSQL with full audit trail
- **Real-time Delivery**: Asynchronous notification delivery via Redis/asynq
- **Multiple Channels**: Support for email, WebSocket, and push notifications
- **Flexible Metadata**: JSON metadata support for rich notification content
- **Priority Levels**: Low, medium, and high priority notifications
- **Expiration**: Optional notification expiration dates
- **Bulk Operations**: Efficient bulk notification creation and management

## Database Schema

### Tables

#### `notifications`
- `id` - UUID primary key
- `user_id` - UUID foreign key to users
- `type` - Notification type enum (system, booking, item, security, reminder, approval)
- `title` - Notification title
- `message` - Notification message content
- `priority` - Priority enum (low, medium, high)
- `read_at` - Timestamp when notification was read (nullable)
- `created_at` - Creation timestamp
- `expires_at` - Optional expiration timestamp
- `related_item_id` - Optional foreign key to items
- `related_booking_id` - Optional foreign key to bookings
- `related_user_id` - Optional foreign key to users
- `metadata` - JSONB for additional data

### Enums

#### `notification_type`
- `system` - System-wide announcements
- `booking` - Booking-related notifications
- `item` - Item-related notifications
- `security` - Security alerts
- `reminder` - Reminder notifications
- `approval` - Approval request notifications

#### `notification_priority`
- `low` - Low priority notifications
- `medium` - Medium priority notifications
- `high` - High priority notifications

## API Endpoints

### User Endpoints

#### GET /notifications
Get user's notifications with optional filtering.

**Query Parameters:**
- `limit` (int32) - Number of notifications to return (default: 20)
- `offset` (int32) - Pagination offset (default: 0)
- `unread` (boolean) - Filter for unread notifications only

**Response:** Array of notification objects

#### GET /notifications/stats
Get notification statistics for the current user.

**Response:**
```json
{
  "unreadCount": 5,
  "totalCount": 25
}
```

#### PUT /notifications/mark-read
Mark notifications as read.

**Request Body:**
```json
{
  "notificationIds": ["uuid1", "uuid2"] // Optional - if omitted, marks all as read
}
```

#### GET /notifications/{id}
Get a specific notification by ID.

**Response:** Single notification object

#### DELETE /notifications/{id}
Delete a specific notification.

### Admin Endpoints

#### POST /admin/notifications
Create a new notification (admin only).

**Request Body:**
```json
{
  "userId": "uuid",
  "type": "system",
  "title": "Notification Title",
  "message": "Notification message content",
  "priority": "medium",
  "expiresAt": "2024-12-31T23:59:59Z", // Optional
  "relatedItemId": "uuid", // Optional
  "relatedBookingId": "uuid", // Optional
  "relatedUserId": "uuid", // Optional
  "metadata": {} // Optional JSON object
}
```

## Service Layer

### `notifications.Service`

The service layer provides the following methods:

```go
// Create a new notification
func (s *Service) CreateNotification(ctx context.Context, params CreateNotificationParams) (*db.Notification, error)

// Get user's notifications with pagination
func (s *Service) GetUserNotifications(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Notification, error)

// Get only unread notifications
func (s *Service) GetUnreadNotifications(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Notification, error)

// Mark notification as read
func (s *Service) MarkAsRead(ctx context.Context, notificationID, userID uuid.UUID) error

// Mark all notifications as read
func (s *Service) MarkAllAsRead(ctx context.Context, userID uuid.UUID) error

// Delete a notification
func (s *Service) DeleteNotification(ctx context.Context, notificationID, userID uuid.UUID) error

// Get notification statistics
func (s *Service) GetNotificationStats(ctx context.Context, userID uuid.UUID) (*NotificationStats, error)

// Clean up expired notifications
func (s *Service) CleanupExpiredNotifications(ctx context.Context) (int64, error)
