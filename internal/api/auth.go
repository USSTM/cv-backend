package api

import (
	"context"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/rbac"
	"golang.org/x/crypto/bcrypt"
)

func (s Server) LoginUser(ctx context.Context, request api.LoginUserRequestObject) (api.LoginUserResponseObject, error) {
	if request.Body == nil {
		return api.LoginUser400JSONResponse(ValidationErr("Request body is required", nil).Create()), nil
	}

	logger := middleware.GetLoggerFromContext(ctx)

	req := *request.Body
	user, err := s.db.Queries().GetUserByEmail(ctx, string(req.Email))
	if err != nil {
		logger.Warn("User not found during login",
			"email", req.Email,
			"error", err)
		return api.LoginUser400JSONResponse(ValidationErr("Invalid email or password.", nil).Create()), nil
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		logger.Warn("Invalid password during login",
			"email", req.Email,
			"error", err)
		return api.LoginUser400JSONResponse(ValidationErr("Invalid email or password.", nil).Create()), nil
	}

	userUUID := user.ID

	token, err := s.jwtService.GenerateToken(ctx, userUUID)
	if err != nil {
		logger.Error("Failed to generate token",
			"email", user.Email,
			"user_id", userUUID,
			"error", err)
		return api.LoginUser500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	logger.Info("User logged in successfully", "email", user.Email)
	return api.LoginUser200JSONResponse{
		Token: &token,
	}, nil
}

func (s Server) PingProtected(ctx context.Context, request api.PingProtectedRequestObject) (api.PingProtectedResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.PingProtected401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewOwnData, nil)
	if err != nil {
		logger.Error("Error checking view_own_data permission",
			"user_id", user.ID,
			"permission", rbac.ViewOwnData,
			"error", err)
		return api.PingProtected500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.PingProtected401JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	return api.PingProtected200JSONResponse{
		Message:   "PONG! Hello " + user.Email,
		Timestamp: time.Now(),
	}, nil
}
