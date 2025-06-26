-- +goose Up
-- Seed default roles
INSERT INTO roles (name, description) VALUES
    ('global_admin', 'Global administrator with system-wide access'),
    ('approver', 'Can approve requests and manage schedules (VP Operations)'),
    ('group_admin', 'Group administrator with group management access'),
    ('member', 'Group member with basic access');

-- Seed basic permissions
INSERT INTO permissions (name, description) VALUES
    ('manage_items', 'Create, update, and delete items globally'),
    ('manage_groups', 'Create, update, and delete groups'),
    ('manage_users', 'Create, update, and delete users globally'),
    ('manage_group_users', 'Add/remove users from specific group'),
    ('approve_all_requests', 'Approve/deny requests across all groups'),
    ('view_group_data', 'View requests/borrowings within specific group'),
    ('view_all_data', 'View requests/borrowings across all groups'),
    ('manage_time_slots', 'Create and manage available time slots'),
    ('manage_all_bookings', 'Manage booking schedules globally'),
    ('manage_group_bookings', 'Manage booking schedules for specific group'),
    ('view_items', 'View item catalog'),
    ('manage_cart', 'Add/remove items from cart'),
    ('request_items', 'Submit requests for borrowing items'),
    ('view_own_data', 'View own requests and borrowings');

-- Assign permissions to roles
-- Global admin gets all permissions
INSERT INTO role_permissions (role_name, permission_name)
SELECT 'global_admin', name FROM permissions;

-- Approver gets approval and scheduling permissions
INSERT INTO role_permissions (role_name, permission_name)
SELECT 'approver', name FROM permissions 
WHERE name IN (
    'approve_all_requests',
    'view_all_data',
    'manage_time_slots',
    'manage_all_bookings',
    'view_items',
    'manage_cart',
    'request_items',
    'view_own_data'
);

-- Group admin gets group management permissions (NO approval rights)
INSERT INTO role_permissions (role_name, permission_name)
SELECT 'group_admin', name FROM permissions 
WHERE name IN (
    'manage_group_users',
    'view_group_data',
    'manage_group_bookings',
    'view_items',
    'manage_cart',
    'request_items',
    'view_own_data'
);

-- Member gets basic permissions
INSERT INTO role_permissions (role_name, permission_name)
SELECT 'member', name FROM permissions 
WHERE name IN (
    'view_items',
    'manage_cart',
    'request_items',
    'view_own_data'
);

-- +goose Down
DELETE FROM role_permissions;
DELETE FROM permissions;
DELETE FROM roles;