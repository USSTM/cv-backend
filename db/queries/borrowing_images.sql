-- name: CreateBorrowingImage :one
INSERT INTO borrowing_images (id, borrowing_id, s3_key, image_type, uploaded_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetBorrowingImageByID :one
SELECT * FROM borrowing_images WHERE id = $1;

-- name: ListBorrowingImagesByBorrowing :many
SELECT * FROM borrowing_images WHERE borrowing_id = $1 ORDER BY image_type ASC, created_at ASC;

-- name: DeleteBorrowingImage :exec
DELETE FROM borrowing_images WHERE id = $1;
