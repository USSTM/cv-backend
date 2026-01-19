-- +goose Up
-- Create enum for notification types
CREATE TYPE notification_type AS ENUM (
    'booking_confirmation',
    'booking_reminder',
    'request_approved',
    'request_denied',
    'item_overdue',
    'item_returned',
    'system_announcement',
    'invitation_received'
);

-- Create enum for notification priority
CREATE TYPE notification_priority AS ENUM ('low', 'normal', 'high');

-- Notifications table
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type notification_type NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    priority notification_priority NOT NULL DEFAULT 'normal',
    read_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NULL,

    -- Optional reference fields for context
    related_booking_id UUID REFERENCES booking(id) ON DELETE CASCADE,
    related_request_id UUID REFERENCES requests(id) ON DELETE CASCADE,
    related_item_id UUID REFERENCES items(id) ON DELETE CASCADE,
    related_user_id UUID REFERENCES users(id) ON DELETE CASCADE,

    -- Additional metadata as JSON
    metadata JSONB
);

-- Indexes for performance
CREATE INDEX idx_notifications_user_unread ON notifications(user_id, created_at DESC) WHERE read_at IS NULL;
CREATE INDEX idx_notifications_user_all ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_type ON notifications(type, created_at DESC);
CREATE INDEX idx_notifications_priority ON notifications(priority, created_at DESC);
CREATE INDEX idx_notifications_expires ON notifications(expires_at) WHERE expires_at IS NOT NULL;

-- Composite index for common queries
CREATE INDEX idx_notifications_user_type_unread ON notifications(user_id, type, created_at DESC) WHERE read_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_notifications_user_type_unread;
DROP INDEX IF EXISTS idx_notifications_expires;
DROP INDEX IF EXISTS idx_notifications_priority;
DROP INDEX IF EXISTS idx_notifications_type;
DROP INDEX IF EXISTS idx_notifications_user_all;
DROP INDEX IF EXISTS idx_notifications_user_unread;
DROP TABLE IF EXISTS notifications;
DROP TYPE IF EXISTS notification_priority;
DROP TYPE IF EXISTS notification_type;
