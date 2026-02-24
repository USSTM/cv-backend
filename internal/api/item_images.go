package api

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	cvimage "github.com/USSTM/cv-backend/internal/image"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/google/uuid"
)

func (s Server) buildItemImageResponse(ctx context.Context, img db.ItemImage) genapi.ItemImage {
	url, _ := s.s3Service.GeneratePresignedURL(ctx, "GET", img.OriginalS3Key, time.Hour)
	thumbURL, _ := s.s3Service.GeneratePresignedURL(ctx, "GET", img.ThumbnailS3Key, time.Hour)
	return genapi.ItemImage{
		Id:           img.ID,
		ItemId:       img.ItemID,
		Url:          url,
		ThumbnailUrl: thumbURL,
		DisplayOrder: int(img.DisplayOrder),
		IsPrimary:    img.IsPrimary,
		CreatedAt:    img.CreatedAt.Time,
	}
}

func (s Server) UploadItemImage(ctx context.Context, request genapi.UploadItemImageRequestObject) (genapi.UploadItemImageResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.UploadItemImage401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageItems, nil)
	if err != nil {
		return genapi.UploadItemImage500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return genapi.UploadItemImage403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	_, err = s.db.Queries().GetItemByID(ctx, request.ItemId)
	if err != nil {
		return genapi.UploadItemImage404JSONResponse(NotFound("Item").Create()), nil
	}

	form, err := request.Body.ReadForm(32 << 20) // 32MB max memory
	if err != nil {
		return genapi.UploadItemImage400JSONResponse(ValidationErr("Failed to parse multipart form", nil).Create()), nil
	}

	files, ok := form.File["image"]
	if !ok || len(files) == 0 {
		return genapi.UploadItemImage400JSONResponse(ValidationErr("Missing image field", nil).Create()), nil
	}
	fileHeader := files[0]
	file, err := fileHeader.Open()
	if err != nil {
		return genapi.UploadItemImage400JSONResponse(ValidationErr("Failed to open image", nil).Create()), nil
	}
	defer file.Close()

	processed, err := cvimage.ValidateAndProcess(file, fileHeader)
	if err != nil {
		return genapi.UploadItemImage400JSONResponse(ValidationErr(err.Error(), nil).Create()), nil
	}

	isPrimary := false
	if vals := form.Value["is_primary"]; len(vals) > 0 {
		isPrimary, _ = strconv.ParseBool(vals[0])
	}

	if isPrimary && !cvimage.IsSquare(processed.Width, processed.Height) {
		return genapi.UploadItemImage400JSONResponse(ValidationErr("Primary image must be square", nil).Create()), nil
	}

	displayOrder := int32(0)
	if vals := form.Value["display_order"]; len(vals) > 0 {
		if n, e := strconv.Atoi(vals[0]); e == nil {
			displayOrder = int32(n)
		}
	}

	ext := "jpg"
	if processed.ContentType == "image/png" {
		ext = "png"
	}
	id := uuid.New()
	shortID := id.String()[:8]
	originalKey := fmt.Sprintf("items/%s/%s-original.%s", request.ItemId, shortID, ext)
	thumbnailKey := fmt.Sprintf("items/%s/%s-thumb.%s", request.ItemId, shortID, ext)

	if err := s.s3Service.PutObject(ctx, originalKey, bytes.NewReader(processed.Original), processed.ContentType); err != nil {
		return genapi.UploadItemImage500JSONResponse(InternalError("Failed to upload image").Create()), nil
	}
	if err := s.s3Service.PutObject(ctx, thumbnailKey, bytes.NewReader(processed.Thumbnail), processed.ContentType); err != nil {
		_ = s.s3Service.DeleteObject(ctx, originalKey)
		return genapi.UploadItemImage500JSONResponse(InternalError("Failed to upload thumbnail").Create()), nil
	}

	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		_ = s.s3Service.DeleteObject(ctx, originalKey)
		_ = s.s3Service.DeleteObject(ctx, thumbnailKey)
		return genapi.UploadItemImage500JSONResponse(InternalError("Failed to start transaction").Create()), nil
	}
	defer tx.Rollback(ctx)
	qtx := s.db.Queries().WithTx(tx)

	if isPrimary {
		if err := qtx.UnsetPrimaryItemImages(ctx, request.ItemId); err != nil {
			_ = s.s3Service.DeleteObject(ctx, originalKey)
			_ = s.s3Service.DeleteObject(ctx, thumbnailKey)
			return genapi.UploadItemImage500JSONResponse(InternalError("Failed to update primary image").Create()), nil
		}
	}

	img, err := qtx.CreateItemImage(ctx, db.CreateItemImageParams{
		ID:             id,
		ItemID:         request.ItemId,
		OriginalS3Key:  originalKey,
		ThumbnailS3Key: thumbnailKey,
		DisplayOrder:   displayOrder,
		IsPrimary:      isPrimary,
		Width:          int32(processed.Width),
		Height:         int32(processed.Height),
		UploadedBy:     &user.ID,
	})
	if err != nil {
		_ = s.s3Service.DeleteObject(ctx, originalKey)
		_ = s.s3Service.DeleteObject(ctx, thumbnailKey)
		return genapi.UploadItemImage500JSONResponse(InternalError("Failed to save image record").Create()), nil
	}

	if err := tx.Commit(ctx); err != nil {
		_ = s.s3Service.DeleteObject(ctx, originalKey)
		_ = s.s3Service.DeleteObject(ctx, thumbnailKey)
		return genapi.UploadItemImage500JSONResponse(InternalError("Failed to commit transaction").Create()), nil
	}

	return genapi.UploadItemImage201JSONResponse(s.buildItemImageResponse(ctx, img)), nil
}

func (s Server) ListItemImages(ctx context.Context, request genapi.ListItemImagesRequestObject) (genapi.ListItemImagesResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.ListItemImages401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewItems, nil)
	if err != nil {
		return genapi.ListItemImages500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return genapi.ListItemImages403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	images, err := s.db.Queries().ListItemImagesByItem(ctx, request.ItemId)
	if err != nil {
		return genapi.ListItemImages500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	var response genapi.ListItemImages200JSONResponse
	for _, img := range images {
		response = append(response, s.buildItemImageResponse(ctx, img))
	}
	return response, nil
}

func (s Server) DeleteItemImage(ctx context.Context, request genapi.DeleteItemImageRequestObject) (genapi.DeleteItemImageResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.DeleteItemImage401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageItems, nil)
	if err != nil {
		return genapi.DeleteItemImage500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return genapi.DeleteItemImage403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	img, err := s.db.Queries().GetItemImageByID(ctx, request.ImageId)
	if err != nil {
		return genapi.DeleteItemImage404JSONResponse(NotFound("Image").Create()), nil
	}

	_ = s.s3Service.DeleteObject(ctx, img.OriginalS3Key)
	_ = s.s3Service.DeleteObject(ctx, img.ThumbnailS3Key)

	if err := s.db.Queries().DeleteItemImage(ctx, img.ID); err != nil {
		return genapi.DeleteItemImage500JSONResponse(InternalError("Failed to delete image record").Create()), nil
	}

	return genapi.DeleteItemImage204Response{}, nil
}

func (s Server) SetItemPrimaryImage(ctx context.Context, request genapi.SetItemPrimaryImageRequestObject) (genapi.SetItemPrimaryImageResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.SetItemPrimaryImage401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageItems, nil)
	if err != nil {
		return genapi.SetItemPrimaryImage500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return genapi.SetItemPrimaryImage403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	img, err := s.db.Queries().GetItemImageByID(ctx, request.ImageId)
	if err != nil {
		return genapi.SetItemPrimaryImage404JSONResponse(NotFound("Image").Create()), nil
	}

	if !cvimage.IsSquare(int(img.Width), int(img.Height)) {
		return genapi.SetItemPrimaryImage400JSONResponse(ValidationErr("Primary image must be square", nil).Create()), nil
	}

	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return genapi.SetItemPrimaryImage500JSONResponse(InternalError("Failed to start transaction").Create()), nil
	}
	defer tx.Rollback(ctx)
	qtx := s.db.Queries().WithTx(tx)

	if err := qtx.UnsetPrimaryItemImages(ctx, img.ItemID); err != nil {
		return genapi.SetItemPrimaryImage500JSONResponse(InternalError("Failed to update primary image").Create()), nil
	}

	if err := qtx.SetItemImageAsPrimary(ctx, img.ID); err != nil {
		return genapi.SetItemPrimaryImage500JSONResponse(InternalError("Failed to set primary image").Create()), nil
	}

	if err := tx.Commit(ctx); err != nil {
		return genapi.SetItemPrimaryImage500JSONResponse(InternalError("Failed to commit transaction").Create()), nil
	}

	img.IsPrimary = true
	return genapi.SetItemPrimaryImage200JSONResponse(s.buildItemImageResponse(ctx, img)), nil
}
