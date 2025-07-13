-- +goose Up
INSERT INTO groups (id, name, description) VALUES
    (gen_random_uuid(), 'usstm', 'Undegraduate Science Society of Toronto Metropolitan University'),
    (gen_random_uuid(), 'pacs', 'Practical Applications of Computer Science');

-- +goose Down
DELETE FROM groups WHERE name = 'usstm';
DELETE FROM groups WHERE name = 'pacs';