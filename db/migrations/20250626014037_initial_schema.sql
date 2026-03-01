-- +goose Up
-- Create enum for item types
CREATE TYPE item_type AS ENUM ('low', 'medium', 'high');

-- Create enum for request status
CREATE TYPE request_status AS ENUM ('pending', 'approved', 'denied', 'fulfilled', 'pending_confirmation', 'confirmed', 'expired', 'no_show', 'cancelled');

-- Create enum for item conditions
CREATE TYPE condition AS ENUM ('unusable', 'damaged', 'decent', 'good', 'pristine');

-- Create enum for scope
CREATE TYPE scope_type AS ENUM ('global', 'group');

-- Groups table
CREATE TABLE groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT
);

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL
);

-- Roles table
CREATE TABLE roles (
    name VARCHAR(255) PRIMARY KEY,
    description TEXT
);

-- Permissions table
CREATE TABLE permissions (
    name VARCHAR(255) PRIMARY KEY,
    description TEXT
);

-- Role permissions junction table
CREATE TABLE role_permissions (
    role_name VARCHAR(255) REFERENCES roles(name) ON DELETE CASCADE,
    permission_name VARCHAR(255) REFERENCES permissions(name) ON DELETE CASCADE,
    PRIMARY KEY (role_name, permission_name)
);

-- User roles table
CREATE TABLE user_roles (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role_name VARCHAR(255) REFERENCES roles(name) ON DELETE CASCADE,
    scope scope_type NOT NULL,
    scope_id UUID REFERENCES groups(id) ON DELETE CASCADE,

    CONSTRAINT scope_consistency CHECK (
        (scope = 'global' AND scope_id IS NULL) OR
        (scope = 'group' AND scope_id IS NOT NULL)
    ),

    UNIQUE (user_id, role_name, scope, scope_id)
);

-- Signup codes table
CREATE TABLE signup_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(32) UNIQUE NOT NULL,
    email VARCHAR(255) NOT NULL,
    role_name VARCHAR(255) NOT NULL REFERENCES roles(name),
    scope scope_type NOT NULL,
    scope_id UUID REFERENCES groups(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    used_at TIMESTAMP NULL,
    expires_at TIMESTAMP NULL,
    created_by UUID NOT NULL REFERENCES users(id),

    CONSTRAINT signup_scope_consistency CHECK (
        (scope = 'global' AND scope_id IS NULL) OR
        (scope = 'group' AND scope_id IS NOT NULL)
    )
);

-- Items table
CREATE TABLE items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type item_type NOT NULL,
    stock INT NOT NULL DEFAULT 0 CHECK (stock >= 0),
    urls TEXT[]
);

-- Cart items table
CREATE TABLE cart (
    group_id UUID REFERENCES groups(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    item_id UUID REFERENCES items(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    quantity INT NOT NULL DEFAULT 1,
    PRIMARY KEY (group_id, user_id, item_id)
);

-- Time Slots for Availability Table
CREATE TABLE time_slots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    CONSTRAINT valid_time_range CHECK (end_time > start_time)
);

-- Availability for Scheduling Table
CREATE TABLE user_availability (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    time_slot_id UUID REFERENCES time_slots(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    CONSTRAINT unique_user_slot_date UNIQUE (user_id, time_slot_id, date)
);

-- Booking System Table for scheduling pickup/return
CREATE TABLE booking (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID REFERENCES users(id) ON DELETE CASCADE,
    manager_id UUID REFERENCES users(id) ON DELETE SET NULL,
    item_id UUID REFERENCES items(id) ON DELETE CASCADE,
    group_id UUID REFERENCES groups(id) ON DELETE CASCADE,
    availability_id UUID REFERENCES user_availability(id) ON DELETE CASCADE,
    pick_up_date TIMESTAMP NOT NULL,
    pick_up_location TEXT NOT NULL,
    return_date TIMESTAMP NOT NULL,
    return_location TEXT NOT NULL,
    status request_status NOT NULL DEFAULT 'pending_confirmation',
    confirmed_at TIMESTAMP,
    confirmed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT valid_dates CHECK (return_date > pick_up_date)
);

-- Requests table for high items needing approval
CREATE TABLE requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    group_id UUID REFERENCES groups(id) ON DELETE CASCADE,
    item_id UUID REFERENCES items(id) ON DELETE CASCADE,
    quantity INT NOT NULL,
    status request_status DEFAULT 'pending',
    requested_at TIMESTAMP DEFAULT NOW(),
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMP,
    fulfilled_at TIMESTAMP NULL,
    booking_id UUID REFERENCES booking(id) ON DELETE SET NULL,
    preferred_availability_id UUID REFERENCES user_availability(id) ON DELETE SET NULL
);

-- Borrowings table for items currently out (medium + approved high)
CREATE TABLE borrowings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    group_id UUID REFERENCES groups(id) ON DELETE CASCADE,
    item_id UUID REFERENCES items(id) ON DELETE CASCADE,
    quantity INT NOT NULL,
    borrowed_at TIMESTAMP DEFAULT NOW(),
    due_date TIMESTAMP,
    returned_at TIMESTAMP,
    before_condition condition NOT NULL,
    before_condition_url TEXT NOT NULL,
    after_condition condition,
    after_condition_url TEXT
);

-- Item takings table for items audit trail (no returns needed)
CREATE TABLE item_takings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    quantity INT NOT NULL,
    taken_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for audit queries on item_takings
CREATE INDEX idx_item_takings_user ON item_takings(user_id, taken_at DESC);
CREATE INDEX idx_item_takings_item ON item_takings(item_id, taken_at DESC);
CREATE INDEX idx_item_takings_group ON item_takings(group_id, taken_at DESC);

-- Booking indexes for performance
CREATE INDEX idx_booking_requester ON booking(requester_id, pick_up_date);
CREATE INDEX idx_booking_status ON booking(status, pick_up_date);
CREATE INDEX idx_booking_manager ON booking(manager_id, pick_up_date);

-- Availability indexes for performance
CREATE INDEX idx_availability_date ON user_availability(date);
CREATE INDEX idx_availability_user ON user_availability(user_id, date);
CREATE INDEX idx_availability_slot ON user_availability(time_slot_id, date);

-- Request booking reference index
CREATE INDEX idx_requests_booking ON requests(booking_id);

-- Notification entity types table (What kind of entity caused the notification)
CREATE TABLE notification_entity_types (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT
);

-- Notification objects table (The event that happened)
CREATE TABLE notification_objects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type_id INT NOT NULL REFERENCES notification_entity_types(id) ON DELETE CASCADE,
    entity_id UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Notification changes table (Who triggered it)
CREATE TABLE notification_changes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_object_id UUID NOT NULL REFERENCES notification_objects(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Notifications table (Who receives it)
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_object_id UUID NOT NULL REFERENCES notification_objects(id) ON DELETE CASCADE,
    notifier_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Notifications indexes
CREATE INDEX idx_notifications_notifier ON notifications(notifier_id, created_at DESC);
CREATE INDEX idx_notifications_is_read ON notifications(notifier_id, is_read);
CREATE INDEX idx_notification_objects_entity ON notification_objects(entity_type_id, entity_id);

-- +goose Down
DROP TABLE IF EXISTS notifications CASCADE;
DROP TABLE IF EXISTS notification_changes CASCADE;
DROP TABLE IF EXISTS notification_objects CASCADE;
DROP TABLE IF EXISTS notification_entity_types CASCADE;
DROP TABLE IF EXISTS booking CASCADE;
DROP TABLE IF EXISTS user_availability CASCADE;
DROP TABLE IF EXISTS time_slots CASCADE;
DROP TABLE IF EXISTS item_takings CASCADE;
DROP TABLE IF EXISTS borrowings CASCADE;
DROP TABLE IF EXISTS requests CASCADE;
DROP TABLE IF EXISTS cart CASCADE;
DROP TABLE IF EXISTS items CASCADE;
DROP TABLE IF EXISTS signup_codes CASCADE;
DROP TABLE IF EXISTS user_roles CASCADE;
DROP TABLE IF EXISTS role_permissions CASCADE;
DROP TABLE IF EXISTS permissions CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS groups CASCADE;
DROP TYPE IF EXISTS scope_type CASCADE;
DROP TYPE IF EXISTS condition CASCADE;
DROP TYPE IF EXISTS request_status CASCADE;
DROP TYPE IF EXISTS item_type CASCADE;
