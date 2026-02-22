-- +goose Up
CREATE TABLE item_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id UUID NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    original_s3_key TEXT NOT NULL,
    thumbnail_s3_key TEXT NOT NULL,
    display_order INT NOT NULL DEFAULT 0,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    width INT NOT NULL,
    height INT NOT NULL,
    uploaded_by UUID REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_item_images_item ON item_images(item_id, display_order);

-- +goose Down
DROP TABLE item_images;
