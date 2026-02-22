-- +goose Up
CREATE TABLE borrowing_images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    borrowing_id UUID NOT NULL REFERENCES borrowings(id) ON DELETE CASCADE,
    s3_key TEXT NOT NULL,
    image_type TEXT NOT NULL CHECK (image_type IN ('before', 'after')),
    uploaded_by UUID REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_borrowing_images_borrowing ON borrowing_images(borrowing_id, image_type);

-- +goose Down
DROP TABLE borrowing_images;
