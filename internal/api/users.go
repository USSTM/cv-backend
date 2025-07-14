package api

import (
	"context"
	"crypto/rand"
	"log"
	"strings"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/google/uuid"
	types "github.com/oapi-codegen/runtime/types"
)

func (s Server) GetUsers(ctx context.Context, request api.GetUsersRequestObject) (api.GetUsersResponseObject, error) {
	// Check permission
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUsers401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_users", nil)
	if err != nil {
		log.Printf("Error checking manage_users permission: %v", err)
		return api.GetUsers500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetUsers403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	users, err := s.db.Queries().GetAllUsers(ctx)
	if err != nil {
		log.Printf("Failed to get users: %v", err)
		return api.GetUsers500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred.",
		}, nil
	}

	// Convert database users to API response format
	var response api.GetUsers200JSONResponse
	for _, user := range users {
		userUUID := user.ID

		// Get user roles from database
		roles, err := s.db.Queries().GetUserRoles(ctx, &user.ID)
		if err != nil {
			log.Printf("Failed to get user roles: %v", err)
		}

		// Default to member role, upgrade if user has higher roles
		role := api.Member
		for _, userRole := range roles {
			switch userRole.RoleName.String {
			case "global_admin":
				role = api.Admin
			case "approver":
				role = api.Approver
			case "group_admin":
				role = api.GroupAdmin
			}
		}

		userResponse := api.User{
			Id:    userUUID,
			Email: types.Email(user.Email),
			Role:  role,
		}
		response = append(response, userResponse)
	}

	return response, nil
}

func (s Server) InviteUser(ctx context.Context, request api.InviteUserRequestObject) (api.InviteUserResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.InviteUser401JSONResponse{Code: 401, Message: "Unauthorized"}, nil
	}
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_group_users", nil)
	if err != nil || !hasPermission {
		return api.InviteUser403JSONResponse{Code: 403, Message: "Insufficient permissions"}, nil
	}

	if request.Body == nil {
		return api.InviteUser400JSONResponse{Code: 400, Message: "Request body is required"}, nil
	}

	req := request.Body
	scopeStr := string(req.Scope)
	
	var scopeID uuid.UUID
	if req.ScopeId != nil {
		scopeID = uuid.UUID(*req.ScopeId)
	}

	if scopeStr == "global" && scopeID != uuid.Nil {
		return api.InviteUser400JSONResponse{Code: 400, Message: "Scope ID must be empty for global scope"}, nil
	}

	if scopeStr == "group" {
		if scopeID == uuid.Nil {
			return api.InviteUser400JSONResponse{Code: 400, Message: "Scope ID must be provided for group scope"}, nil
		}

		// check if the group exists with error handling, if it does exist, then do nothing
		_, err := s.db.Queries().GetGroupByID(ctx, scopeID)
		if err != nil {
			return api.InviteUser404JSONResponse{Code: 404, Message: "Group not found"}, nil
		}
	}

	// generates a random code for the sign-up link (just a random string of 32 characters)
	code, err := generateRandomCode(32)
	if err != nil {
		return api.InviteUser500JSONResponse{Code: 500, Message: "Failed to generate sign-up code"}, nil
	}

	// this makes it so that if the scopeID is uuid.Nil, it will be nil in the database (instead of 0000000-0000-0000-0000-000000000000)
	var scopeIDPtr *uuid.UUID
	if scopeID != uuid.Nil {
		scopeIDPtr = &scopeID
	} else {
		scopeIDPtr = nil
	}

	params := db.CreateSignUpCodeParams{
		Code:      code,
		Email:     string(req.Email),
		RoleName:  req.RoleName,
		Scope:     db.ScopeType(scopeStr),
		ScopeID:   scopeIDPtr,
		CreatedBy: user.ID,
	}

	signupCode, err := s.db.Queries().CreateSignUpCode(ctx, params)
	if err != nil {
		return api.InviteUser500JSONResponse{Code: 500, Message: "An unexpected error occurred."}, nil
	}

	return api.InviteUser201JSONResponse{Code: &signupCode.Code}, nil
}

func generateRandomCode(length int) (string, error) {
	const characters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	if length <= 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.Grow(length)

	for i := 0; i < length; i++ {
		b := make([]byte, 1)
		_, err := rand.Read(b)
		if err != nil {
			return "", err
		}
		sb.WriteByte(characters[int(b[0])%len(characters)])
	}

	return sb.String(), nil
}
