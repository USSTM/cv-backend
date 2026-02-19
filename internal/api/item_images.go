package api

import (
	"context"

	genapi "github.com/USSTM/cv-backend/generated/api"
)

func (s Server) UploadItemImage(ctx context.Context, request genapi.UploadItemImageRequestObject) (genapi.UploadItemImageResponseObject, error) {
	return genapi.UploadItemImage500JSONResponse(InternalError("not implemented").Create()), nil
}

func (s Server) ListItemImages(ctx context.Context, request genapi.ListItemImagesRequestObject) (genapi.ListItemImagesResponseObject, error) {
	return genapi.ListItemImages500JSONResponse(InternalError("not implemented").Create()), nil
}

func (s Server) DeleteItemImage(ctx context.Context, request genapi.DeleteItemImageRequestObject) (genapi.DeleteItemImageResponseObject, error) {
	return genapi.DeleteItemImage500JSONResponse(InternalError("not implemented").Create()), nil
}

func (s Server) SetItemPrimaryImage(ctx context.Context, request genapi.SetItemPrimaryImageRequestObject) (genapi.SetItemPrimaryImageResponseObject, error) {
	return genapi.SetItemPrimaryImage500JSONResponse(InternalError("not implemented").Create()), nil
}
