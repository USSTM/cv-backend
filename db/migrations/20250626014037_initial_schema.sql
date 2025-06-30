-- +goose Up
-- Create enum for item types
CREATE TYPE item_type AS ENUM ('low', 'medium', 'high');

-- Create enum for request status
CREATE TYPE request_status AS ENUM ('pending', 'approved', 'denied');

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
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL
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
    reviewed_at TIMESTAMP
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

-- Time Slots for Availability Table
CREATE TABLE time_slots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL
);

-- Availability for Scheduling Table
CREATE TABLE user_availability (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    group_id UUID REFERENCES groups(id) ON DELETE CASCADE,
    time_slot_id UUID REFERENCES time_slots(id) ON DELETE CASCADE,
    date DATE NOT NULL
);

-- Booking System Table for scheduling pickup/return
CREATE TABLE booking (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID REFERENCES users(id) ON DELETE CASCADE,
    manager_id UUID REFERENCES users(id) ON DELETE SET NULL,
    item_id UUID REFERENCES items(id) ON DELETE CASCADE,
    availability_id UUID REFERENCES user_availability(id) ON DELETE CASCADE,
    confirmed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    pick_up_date TIMESTAMP NOT NULL,
    pick_up_location TEXT NOT NULL,
    return_date TIMESTAMP NOT NULL,
    return_location TEXT NOT NULL,
    status request_status NOT NULL DEFAULT 'pending',
    CONSTRAINT valid_dates CHECK (return_date > pick_up_date)
);

-- +goose Down
DROP TABLE IF EXISTS booking;
DROP TABLE IF EXISTS user_availability;
DROP TABLE IF EXISTS time_slots;
DROP TABLE IF EXISTS borrowings;
DROP TABLE IF EXISTS requests;
DROP TABLE IF EXISTS cart;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS signup_codes;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS groups;
DROP TYPE IF EXISTS scope_type;
DROP TYPE IF EXISTS condition;
DROP TYPE IF EXISTS request_status;
DROP TYPE IF EXISTS item_type;