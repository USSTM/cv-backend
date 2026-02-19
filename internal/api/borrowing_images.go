package api

import (
	"context"

	genapi "github.com/USSTM/cv-backend/generated/api"
)

func (s Server) UploadBorrowingImage(ctx context.Context, request genapi.UploadBorrowingImageRequestObject) (genapi.UploadBorrowingImageResponseObject, error) {
	return genapi.UploadBorrowingImage500JSONResponse(InternalError("not implemented").Create()), nil
}

func (s Server) ListBorrowingImages(ctx context.Context, request genapi.ListBorrowingImagesRequestObject) (genapi.ListBorrowingImagesResponseObject, error) {
	return genapi.ListBorrowingImages500JSONResponse(InternalError("not implemented").Create()), nil
}

func (s Server) DeleteBorrowingImage(ctx context.Context, request genapi.DeleteBorrowingImageRequestObject) (genapi.DeleteBorrowingImageResponseObject, error) {
	return genapi.DeleteBorrowingImage500JSONResponse(InternalError("not implemented").Create()), nil
}
