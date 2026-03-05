-- +goose Up
ALTER TABLE users ADD COLUMN preferences JSONB NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE users DROP COLUMN preferences;
