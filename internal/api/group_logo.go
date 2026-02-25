package api

import (
	"bytes"
	"context"
	"fmt"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	cvimage "github.com/USSTM/cv-backend/internal/image"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s Server) UploadGroupLogo(ctx context.Context, request genapi.UploadGroupLogoRequestObject) (genapi.UploadGroupLogoResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.UploadGroupLogo401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	groupID := request.GroupId
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageGroupUsers, &groupID)
	if err != nil {
		return genapi.UploadGroupLogo500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return genapi.UploadGroupLogo403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	group, err := s.db.Queries().GetGroupByID(ctx, groupID)
	if err != nil {
		return genapi.UploadGroupLogo404JSONResponse(NotFound("Group").Create()), nil
	}

	form, err := request.Body.ReadForm(32 << 20)
	if err != nil {
		return genapi.UploadGroupLogo400JSONResponse(ValidationErr("Failed to parse multipart form", nil).Create()), nil
	}

	files, ok := form.File["image"]
	if !ok || len(files) == 0 {
		return genapi.UploadGroupLogo400JSONResponse(ValidationErr("Missing image field", nil).Create()), nil
	}
	fileHeader := files[0]
	file, err := fileHeader.Open()
	if err != nil {
		return genapi.UploadGroupLogo400JSONResponse(ValidationErr("Failed to open image", nil).Create()), nil
	}
	defer file.Close()

	processed, err := cvimage.ValidateAndProcess(file, fileHeader)
	if err != nil {
		return genapi.UploadGroupLogo400JSONResponse(ValidationErr(err.Error(), nil).Create()), nil
	}

	if !cvimage.IsSquare(processed.Width, processed.Height) {
		return genapi.UploadGroupLogo400JSONResponse(ValidationErr("Group logo must be square", nil).Create()), nil
	}

	oldLogoKey := group.LogoS3Key
	oldThumbKey := group.LogoThumbnailS3Key

	ext := "jpg"
	if processed.ContentType == "image/png" {
		ext = "png"
	}
	logoID := uuid.New().String()
	originalKey := fmt.Sprintf("groups/%s/%s-logo-original.%s", groupID, logoID, ext)
	thumbnailKey := fmt.Sprintf("groups/%s/%s-logo-thumb.%s", groupID, logoID, ext)

	logger := middleware.GetLoggerFromContext(ctx)

	if err := s.s3Service.PutObject(ctx, originalKey, bytes.NewReader(processed.Original), processed.ContentType); err != nil {
		return genapi.UploadGroupLogo500JSONResponse(InternalError("Failed to upload logo").Create()), nil
	}
	if err := s.s3Service.PutObject(ctx, thumbnailKey, bytes.NewReader(processed.Thumbnail), processed.ContentType); err != nil {
		if err := s.s3Service.DeleteObject(ctx, originalKey); err != nil {
			logger.Warn("failed to delete S3 object", "key", originalKey, "error", err)
		}
		return genapi.UploadGroupLogo500JSONResponse(InternalError("Failed to upload logo thumbnail").Create()), nil
	}

	updated, err := s.db.Queries().UpdateGroupLogo(ctx, db.UpdateGroupLogoParams{
		ID:                 groupID,
		LogoS3Key:          pgtype.Text{String: originalKey, Valid: true},
		LogoThumbnailS3Key: pgtype.Text{String: thumbnailKey, Valid: true},
	})
	if err != nil {
		if err := s.s3Service.DeleteObject(ctx, originalKey); err != nil {
			logger.Warn("failed to delete S3 object", "key", originalKey, "error", err)
		}
		if err := s.s3Service.DeleteObject(ctx, thumbnailKey); err != nil {
			logger.Warn("failed to delete S3 object", "key", thumbnailKey, "error", err)
		}
		return genapi.UploadGroupLogo500JSONResponse(InternalError("Failed to save logo record").Create()), nil
	}

	if oldLogoKey.Valid {
		if err := s.s3Service.DeleteObject(ctx, oldLogoKey.String); err != nil {
			logger.Warn("failed to delete S3 object", "key", oldLogoKey.String, "error", err)
		}
	}
	if oldThumbKey.Valid {
		if err := s.s3Service.DeleteObject(ctx, oldThumbKey.String); err != nil {
			logger.Warn("failed to delete S3 object", "key", oldThumbKey.String, "error", err)
		}
	}

	logoURL, thumbURL := s.resolveGroupLogoURLs(ctx, updated)
	var desc *string
	if updated.Description.Valid {
		desc = &updated.Description.String
	}
	return genapi.UploadGroupLogo200JSONResponse(genapi.Group{
		Id:               updated.ID,
		Name:             updated.Name,
		Description:      desc,
		LogoUrl:          logoURL,
		LogoThumbnailUrl: thumbURL,
	}), nil
}
