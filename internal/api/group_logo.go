package api

import (
	"bytes"
	"context"
	"fmt"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	cvimage "github.com/USSTM/cv-backend/internal/image"
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

	// Delete old logo from S3 if it exists
	if group.LogoS3Key.Valid {
		_ = s.s3Service.DeleteObject(ctx, group.LogoS3Key.String)
	}
	if group.LogoThumbnailS3Key.Valid {
		_ = s.s3Service.DeleteObject(ctx, group.LogoThumbnailS3Key.String)
	}

	ext := "jpg"
	if processed.ContentType == "image/png" {
		ext = "png"
	}
	shortID := uuid.New().String()[:8]
	originalKey := fmt.Sprintf("groups/%s/%s-logo-original.%s", groupID, shortID, ext)
	thumbnailKey := fmt.Sprintf("groups/%s/%s-logo-thumb.%s", groupID, shortID, ext)

	if err := s.s3Service.PutObject(ctx, originalKey, bytes.NewReader(processed.Original), processed.ContentType); err != nil {
		return genapi.UploadGroupLogo500JSONResponse(InternalError("Failed to upload logo").Create()), nil
	}
	if err := s.s3Service.PutObject(ctx, thumbnailKey, bytes.NewReader(processed.Thumbnail), processed.ContentType); err != nil {
		_ = s.s3Service.DeleteObject(ctx, originalKey)
		return genapi.UploadGroupLogo500JSONResponse(InternalError("Failed to upload logo thumbnail").Create()), nil
	}

	updated, err := s.db.Queries().UpdateGroupLogo(ctx, db.UpdateGroupLogoParams{
		ID:                 groupID,
		LogoS3Key:          pgtype.Text{String: originalKey, Valid: true},
		LogoThumbnailS3Key: pgtype.Text{String: thumbnailKey, Valid: true},
	})
	if err != nil {
		_ = s.s3Service.DeleteObject(ctx, originalKey)
		_ = s.s3Service.DeleteObject(ctx, thumbnailKey)
		return genapi.UploadGroupLogo500JSONResponse(InternalError("Failed to save logo record").Create()), nil
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
