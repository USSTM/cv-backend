package api

import (
	"github.com/USSTM/cv-backend/internal/rbac"
	"context"
	"log"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

func (s Server) LoginUser(ctx context.Context, request api.LoginUserRequestObject) (api.LoginUserResponseObject, error) {
	if request.Body == nil {
		return api.LoginUser400JSONResponse{
			Code:    400,
			Message: "Request body is required",
		}, nil
	}

	req := *request.Body
	user, err := s.db.Queries().GetUserByEmail(ctx, string(req.Email))
	if err != nil {
		log.Printf("User not found: %v", err)
		return api.LoginUser400JSONResponse{
			Code:    400,
			Message: "Invalid email or password.",
		}, nil
	}

	// TODO: Add password field to LoginRequest schema
	// For now, validate against a hardcoded password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("password")); err != nil {
		log.Printf("Invalid password: %v", err)
		return api.LoginUser400JSONResponse{
			Code:    400,
			Message: "Invalid email or password.",
		}, nil
	}

	userUUID := user.ID

	token, err := s.jwtService.GenerateToken(ctx, userUUID)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		return api.LoginUser500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred.",
		}, nil
	}

	log.Printf("User logged in: %s", user.Email)
	return api.LoginUser200JSONResponse{
		Token: &token,
	}, nil
}

func (s Server) PingProtected(ctx context.Context, request api.PingProtectedRequestObject) (api.PingProtectedResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.PingProtected401JSONResponse{
			Code:    401,
			Message: "Unauthorized!",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewOwnData, nil)
	if err != nil {
		log.Printf("Error checking view_own_data permission: %v", err)
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
