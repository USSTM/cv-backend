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
		return api.LoginUser400JSONResponse{
			Code:    400,
			Message: "Request body is required",
		}, nil
	}

	logger := middleware.GetLoggerFromContext(ctx)

	req := *request.Body
	user, err := s.db.Queries().GetUserByEmail(ctx, string(req.Email))
	if err != nil {
		logger.Warn("User not found during login",
			"email", req.Email,
			"error", err)
		return api.LoginUser400JSONResponse{
			Code:    400,
			Message: "Invalid email or password.",
		}, nil
	}

	// TODO: Add password field to LoginRequest schema
	// For now, validate against a hardcoded password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("password")); err != nil {
		logger.Warn("Invalid password during login",
			"email", req.Email,
			"error", err)
		return api.LoginUser400JSONResponse{
			Code:    400,
			Message: "Invalid email or password.",
		}, nil
	}

	userUUID := user.ID

	token, err := s.jwtService.GenerateToken(ctx, userUUID)
	if err != nil {
		logger.Error("Failed to generate token",
			"email", user.Email,
			"user_id", userUUID,
			"error", err)
		return api.LoginUser500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred.",
		}, nil
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
		return api.PingProtected401JSONResponse{
			Code:    401,
			Message: "Unauthorized!",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewOwnData, nil)
	if err != nil {
		logger.Error("Error checking view_own_data permission",
			"user_id", user.ID,
			"permission", rbac.ViewOwnData,
			"error", err)
		return api.PingProtected500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.PingProtected401JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	return api.PingProtected200JSONResponse{
		Message:   "PONG! Hello " + user.Email,
		Timestamp: time.Now(),
	}, nil
}
