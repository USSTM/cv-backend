package api

import (
	"bytes"
	"context"
	"fmt"
	"time"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	cvimage "github.com/USSTM/cv-backend/internal/image"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s Server) buildBorrowingImageResponse(ctx context.Context, img db.BorrowingImage) genapi.BorrowingImage {
	logger := middleware.GetLoggerFromContext(ctx)
	url, err := s.s3Service.GeneratePresignedURL(ctx, "GET", img.S3Key, time.Hour)
	if err != nil {
		logger.Warn("failed to generate presigned URL", "key", img.S3Key, "error", err)
	}
	return genapi.BorrowingImage{
		Id:          img.ID,
		BorrowingId: img.BorrowingID,
		Url:         url,
		ImageType:   genapi.BorrowingImageImageType(img.ImageType),
		CreatedAt:   img.CreatedAt.Time,
	}
}

// returns borrowing and whether the user may manage images for it.
// users with manage_all_bookings may access any borrowing; others need request_items and must 'own' it.
func (s Server) checkBorrowingAccess(ctx context.Context, userID, borrowingID uuid.UUID) (db.Borrowing, bool, error) {
	canManageAll, err := s.authenticator.CheckPermission(ctx, userID, rbac.ManageAllBookings, nil)
	if err != nil {
		return db.Borrowing{}, false, err
	}
	if canManageAll {
		borrowing, err := s.db.Queries().GetBorrowingByID(ctx, borrowingID)
		if err != nil {
			return db.Borrowing{}, false, err
		}
		return borrowing, true, nil
	}

	canOwn, err := s.authenticator.CheckPermission(ctx, userID, rbac.RequestItems, nil)
	if err != nil {
		return db.Borrowing{}, false, err
	}
	if !canOwn {
		return db.Borrowing{}, false, nil
	}

	borrowing, err := s.db.Queries().GetBorrowingByID(ctx, borrowingID)
	if err != nil {
		return db.Borrowing{}, false, err
	}
	return borrowing, borrowing.UserID != nil && *borrowing.UserID == userID, nil
}

func (s Server) UploadBorrowingImage(ctx context.Context, request genapi.UploadBorrowingImageRequestObject) (genapi.UploadBorrowingImageResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.UploadBorrowingImage401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	borrowing, allowed, err := s.checkBorrowingAccess(ctx, user.ID, request.BorrowingId)
	if err == pgx.ErrNoRows {
		return genapi.UploadBorrowingImage404JSONResponse(NotFound("Borrowing").Create()), nil
	}
	if err != nil {
		return genapi.UploadBorrowingImage500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !allowed {
		return genapi.UploadBorrowingImage403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	form, err := request.Body.ReadForm(32 << 20)
	if err != nil {
		return genapi.UploadBorrowingImage400JSONResponse(ValidationErr("Failed to parse multipart form", nil).Create()), nil
	}

	imageTypeVals := form.Value["image_type"]
	if len(imageTypeVals) == 0 || imageTypeVals[0] == "" {
		return genapi.UploadBorrowingImage400JSONResponse(ValidationErr("Missing image_type field", nil).Create()), nil
	}
	imageType := imageTypeVals[0]
	if imageType != "before" && imageType != "after" {
		return genapi.UploadBorrowingImage400JSONResponse(ValidationErr("image_type must be 'before' or 'after'", nil).Create()), nil
	}
	// after-images may be uploaded before return to document condition in advance
	if imageType == "before" && borrowing.ReturnedAt.Valid {
		return genapi.UploadBorrowingImage400JSONResponse(ValidationErr("Cannot upload before-image for a returned borrowing", nil).Create()), nil
	}

	files, ok := form.File["image"]
	if !ok || len(files) == 0 {
		return genapi.UploadBorrowingImage400JSONResponse(ValidationErr("Missing image field", nil).Create()), nil
	}
	fileHeader := files[0]
	file, err := fileHeader.Open()
	if err != nil {
		return genapi.UploadBorrowingImage400JSONResponse(ValidationErr("Failed to open image", nil).Create()), nil
	}
	defer file.Close()

	processed, err := cvimage.ValidateAndProcess(file, fileHeader)
	if err != nil {
		return genapi.UploadBorrowingImage400JSONResponse(ValidationErr(err.Error(), nil).Create()), nil
	}

	ext := "jpg"
	if processed.ContentType == "image/png" {
		ext = "png"
	}
	id := uuid.New()
	s3Key := fmt.Sprintf("borrowings/%s/%s-%s.%s", request.BorrowingId, id.String()[:8], imageType, ext)

	if err := s.s3Service.PutObject(ctx, s3Key, bytes.NewReader(processed.Original), processed.ContentType); err != nil {
		return genapi.UploadBorrowingImage500JSONResponse(InternalError("Failed to upload image").Create()), nil
	}

	img, err := s.db.Queries().CreateBorrowingImage(ctx, db.CreateBorrowingImageParams{
		ID:          id,
		BorrowingID: request.BorrowingId,
		S3Key:       s3Key,
		ImageType:   imageType,
		UploadedBy:  &user.ID,
	})
	if err != nil {
		_ = s.s3Service.DeleteObject(ctx, s3Key)
		return genapi.UploadBorrowingImage500JSONResponse(InternalError("Failed to save image record").Create()), nil
	}

	return genapi.UploadBorrowingImage201JSONResponse(s.buildBorrowingImageResponse(ctx, img)), nil
}

func (s Server) ListBorrowingImages(ctx context.Context, request genapi.ListBorrowingImagesRequestObject) (genapi.ListBorrowingImagesResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.ListBorrowingImages401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	_, allowed, err := s.checkBorrowingAccess(ctx, user.ID, request.BorrowingId)
	if err == pgx.ErrNoRows {
		return genapi.ListBorrowingImages404JSONResponse(NotFound("Borrowing").Create()), nil
	}
	if err != nil {
		return genapi.ListBorrowingImages500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !allowed {
		return genapi.ListBorrowingImages403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	images, err := s.db.Queries().ListBorrowingImagesByBorrowing(ctx, request.BorrowingId)
	if err != nil {
		return genapi.ListBorrowingImages500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	var response genapi.ListBorrowingImages200JSONResponse
	for _, img := range images {
		response = append(response, s.buildBorrowingImageResponse(ctx, img))
	}
	return response, nil
}

func (s Server) DeleteBorrowingImage(ctx context.Context, request genapi.DeleteBorrowingImageRequestObject) (genapi.DeleteBorrowingImageResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.DeleteBorrowingImage401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	_, allowed, err := s.checkBorrowingAccess(ctx, user.ID, request.BorrowingId)
	if err == pgx.ErrNoRows {
		return genapi.DeleteBorrowingImage404JSONResponse(NotFound("Borrowing").Create()), nil
	}
	if err != nil {
		return genapi.DeleteBorrowingImage500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !allowed {
		return genapi.DeleteBorrowingImage403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	img, err := s.db.Queries().GetBorrowingImageByID(ctx, request.ImageId)
	if err != nil {
		return genapi.DeleteBorrowingImage404JSONResponse(NotFound("Image").Create()), nil
	}
	if img.BorrowingID != request.BorrowingId {
		return genapi.DeleteBorrowingImage404JSONResponse(NotFound("Image").Create()), nil
	}

	if err := s.db.Queries().DeleteBorrowingImage(ctx, img.ID); err != nil {
		return genapi.DeleteBorrowingImage500JSONResponse(InternalError("Failed to delete image record").Create()), nil
	}
	_ = s.s3Service.DeleteObject(ctx, img.S3Key)

	return genapi.DeleteBorrowingImage204Response{}, nil
}
