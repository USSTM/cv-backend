-- Create enum for user roles
CREATE TYPE user_role AS ENUM ('admin', 'organization', 'member');

-- Create enum for group role
CREATE TYPE group_role AS ENUM ('manage', 'member');

-- Create enum for item types
CREATE TYPE item_type AS ENUM ('low', 'medium', 'high');

-- Create enum for request status
CREATE TYPE request_status AS ENUM ('pending', 'approved', 'denied');

-- Create enum for item conditions 
CREATE TYPE condition AS ENUM ('unusable', 'damaged', 'decent', 'good', 'pristine');

-- Create enum for condition type
CREATE TYPE condition_type AS ENUM ('file', 'image');

-- Create enum for user actions 
CREATE TYPE user_action AS ENUM ('borrow', 'return', 'request', 'rejected', 'approved');

-- Student Groups table 
CREATE TABLE student_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL
);

-- Users table 
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NULL,
    role user_role NOT NULL

   CHECK (
        (role = 'organization' AND password_hash IS NULL) OR 
        (role != 'organization' AND password_hash IS NOT NULL)
    )
);

-- User groups table
CREATE TABLE user_groups (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    group_id UUID REFERENCES student_groups(id) ON DELETE CASCADE,
    role group_role NOT NULL,
    PRIMARY KEY (user_id, group_id)
);


-- Items table 
CREATE TABLE items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    type item_type NOT NULL,
    stock INT NOT NULL DEFAULT 0,
    urls TEXT[] NULL
);

-- Cart items table 
CREATE TABLE cart (
    group_id UUID REFERENCES student_groups(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    item_id UUID REFERENCES items(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    quantity INT NOT NULL DEFAULT 1,
    PRIMARY KEY (group_id, user_id, item_id)
);

-- Audit Log Table
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID REFERENCES items(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    group_id UUID REFERENCES student_groups(id) ON DELETE CASCADE,
    status user_action NOT NULL,
    date TIMESTAMP NOT NULL DEFAULT NOW(),
    date_to TIMESTAMP NULL,
    condition_id UUID NULL,
    approver_id UUID REFERENCES users(id) ON DELETE SET NULL,
    notes TEXT
);

-- Item Condition Table
CREATE TABLE item_condition (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    before condition NOT NULL,
    after condition NULL,
    before_image_url TEXT NOT NULL,
    after_image_url TEXT,
    type condition_type NOT NULL,
    audit_log_id UUID REFERENCES audit_log(id) ON DELETE CASCADE,
    recorded_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Add foreign key constraint after both tables exist
ALTER TABLE audit_log ADD CONSTRAINT fk_audit_log_condition 
    FOREIGN KEY (condition_id) REFERENCES item_condition(id) ON DELETE SET NULL;

-- View for Borrowed Items
CREATE VIEW borrowed AS
SELECT * FROM audit_log WHERE status = 'borrow';

-- View for Returned Items
CREATE VIEW returned AS
SELECT * FROM audit_log WHERE status = 'return';

-- View for Requested Items
CREATE VIEW requested AS
SELECT * FROM audit_log WHERE status = 'request';

-- View for Rejected Items
CREATE VIEW rejected AS
SELECT * FROM audit_log WHERE status = 'rejected';

-- View for High Request Items
CREATE VIEW high_item_requests AS
SELECT a.*, i.name as item_name, i.type 
FROM audit_log a 
JOIN items i ON a.item_id = i.id
WHERE i.type = 'high' AND a.status = 'request';

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
    group_id UUID REFERENCES student_groups(id) ON DELETE CASCADE,
    time_slot_id UUID REFERENCES time_slots(id) ON DELETE CASCADE, -- time for day
    date DATE NOT NULL -- specifies date on calendar
);

-- Booking System Table
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