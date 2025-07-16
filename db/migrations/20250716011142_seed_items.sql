-- +goose Up
INSERT INTO items (id, name, description, type, stock)
VALUES
    (gen_random_uuid(), 'Laptop', 'Dell XPS 13', 'high', 10),
    (gen_random_uuid(), 'Projector', 'Epson HD Projector', 'medium', 5),
    (gen_random_uuid(), 'HDMI Cable', '2m HDMI cable', 'low', 50),
    (gen_random_uuid(), 'Whiteboard', 'Magnetic whiteboard', 'medium', 3),
    (gen_random_uuid(), 'Tablet', 'iPad Air', 'high', 7);


-- +goose Down
DELETE FROM items WHERE name IN ('Laptop', 'Projector', 'HDMI Cable', 'Whiteboard', 'Tablet');