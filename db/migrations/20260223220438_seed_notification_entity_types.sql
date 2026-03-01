-- +goose Up
-- +goose StatementBegin
-- Seed basic notification entity types
INSERT INTO notification_entity_types (name, description) VALUES
    ('system', 'System notifications'),
    ('general', 'General announcements'),
    ('request', 'Item request updates'),
    ('booking', 'Booking and scheduling updates'),
    ('item_returned', 'When an item is returned');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM notification_entity_types WHERE name IN ('system', 'general', 'request', 'booking', 'item_returned');
-- +goose StatementEnd
