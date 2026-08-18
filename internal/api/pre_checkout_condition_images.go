package api

import (
	"bytes"
	"context"
	"fmt"
	"time"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
	cvimage "github.com/USSTM/cv-backend/internal/image"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UploadPreCheckoutConditionImage stores a condition image before a borrowing
// exists. The returned key is persisted by CheckoutCart as beforeConditionUrl.
func (s Server) UploadPreCheckoutConditionImage(ctx context.Context, request genapi.UploadPreCheckoutConditionImageRequestObject) (genapi.UploadPreCheckoutConditionImageResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.UploadPreCheckoutConditionImage401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	form, err := request.Body.ReadForm(32 << 20)
	if err != nil {
		return genapi.UploadPreCheckoutConditionImage400JSONResponse(ValidationErr("Failed to parse multipart form", nil).Create()), nil
	}
	itemIDValues := form.Value["item_id"]
	if len(itemIDValues) == 0 || itemIDValues[0] == "" {
		return genapi.UploadPreCheckoutConditionImage400JSONResponse(ValidationErr("Missing item_id field", nil).Create()), nil
	}
	itemID, err := uuid.Parse(itemIDValues[0])
	if err != nil {
		return genapi.UploadPreCheckoutConditionImage400JSONResponse(ValidationErr("item_id must be a valid UUID", nil).Create()), nil
	}
	if _, err := s.db.Queries().GetItemByID(ctx, itemID); err == pgx.ErrNoRows {
		return genapi.UploadPreCheckoutConditionImage404JSONResponse(NotFound("Item").Create()), nil
	} else if err != nil {
		return genapi.UploadPreCheckoutConditionImage500JSONResponse(InternalError("Failed to load item").Create()), nil
	}
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewItems, nil)
	if err != nil {
		return genapi.UploadPreCheckoutConditionImage500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return genapi.UploadPreCheckoutConditionImage403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	files, ok := form.File["image"]
	if !ok || len(files) == 0 {
		return genapi.UploadPreCheckoutConditionImage400JSONResponse(ValidationErr("Missing image field", nil).Create()), nil
	}
	fileHeader := files[0]
	file, err := fileHeader.Open()
	if err != nil {
		return genapi.UploadPreCheckoutConditionImage400JSONResponse(ValidationErr("Failed to open image", nil).Create()), nil
	}
	defer file.Close()

	processed, err := cvimage.ValidateAndProcess(file, fileHeader)
	if err != nil {
		return genapi.UploadPreCheckoutConditionImage400JSONResponse(ValidationErr(err.Error(), nil).Create()), nil
	}

	ext := "jpg"
	if processed.ContentType == "image/png" {
		ext = "png"
	}
	key := fmt.Sprintf("pre-checkout-condition-images/%s/%s/%s.%s", user.ID, itemID, uuid.New(), ext)
	if err := s.s3Service.PutObject(ctx, key, bytes.NewReader(processed.Original), processed.ContentType); err != nil {
		return genapi.UploadPreCheckoutConditionImage500JSONResponse(InternalError("Failed to upload image").Create()), nil
	}
	previewURL, err := s.s3Service.GeneratePresignedURL(ctx, "GET", key, time.Hour)
	if err != nil {
		// Best-effort cleanup: without a preview, the uploaded image is unusable.
		_ = s.s3Service.DeleteObject(ctx, key)
		return genapi.UploadPreCheckoutConditionImage500JSONResponse(InternalError("Failed to generate image preview").Create()), nil
	}

	return genapi.UploadPreCheckoutConditionImage201JSONResponse(genapi.PreCheckoutConditionImage{
		ItemId:             itemID,
		BeforeConditionUrl: key,
		PreviewUrl:         previewURL,
	}), nil
}
