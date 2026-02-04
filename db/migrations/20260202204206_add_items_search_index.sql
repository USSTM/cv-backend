-- +goose Up
CREATE INDEX idx_items_search ON items
USING gin(to_tsvector('english', name || ' ' || COALESCE(description, '')));

-- +goose Down
DROP INDEX IF EXISTS idx_items_search;
