-- +goose Up
ALTER TABLE requests
    ADD COLUMN requested_return_at TIMESTAMP;

-- +goose Down
ALTER TABLE requests
    DROP COLUMN requested_return_at;
