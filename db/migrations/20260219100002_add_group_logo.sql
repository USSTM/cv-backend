-- +goose Up
ALTER TABLE groups ADD COLUMN logo_s3_key TEXT;
ALTER TABLE groups ADD COLUMN logo_thumbnail_s3_key TEXT;

-- +goose Down
ALTER TABLE groups DROP COLUMN logo_thumbnail_s3_key;
ALTER TABLE groups DROP COLUMN logo_s3_key;
