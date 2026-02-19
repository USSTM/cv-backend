package api

import (
	"context"

	genapi "github.com/USSTM/cv-backend/generated/api"
)

func (s Server) UploadGroupLogo(ctx context.Context, request genapi.UploadGroupLogoRequestObject) (genapi.UploadGroupLogoResponseObject, error) {
	return genapi.UploadGroupLogo500JSONResponse(InternalError("not implemented").Create()), nil
}
