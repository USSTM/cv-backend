package api

import (
	"context"
	"encoding/json"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/google/uuid"
	"log"
	"net/http"
	"strings"

	"crypto/rand"
	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
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


func (s Server) InviteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 401, Message: "Unauthorized"})
		return
	}
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_group_users", nil)
	if err != nil || !hasPermission {
		w.WriteHeader(403)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 403, Message: "Insufficient permissions"})
		return
	}

	var req struct {
		Email    openapi_types.Email `json:"email"`
		RoleName string              `json:"role_name"`
		Scope    string              `json:"scope"`
		ScopeID  uuid.UUID           `json:"scope_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 400, Message: "Invalid request body"})
		return
	}

	if req.Scope == "global" && req.ScopeID != uuid.Nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 400, Message: "Scope ID must be empty for global scope"})
		return
	}

	if req.Scope == "group" {
		if req.ScopeID == uuid.Nil {
			w.WriteHeader(400)
			_ = json.NewEncoder(w).Encode(api.Error{Code: 400, Message: "Scope ID must be provided for group scope"})
			return
		}

		// check if the group exists with error handling, if it does exist, then do nothing
		_, err := s.db.Queries().GetGroupByID(ctx, req.ScopeID)
		if err != nil {
			w.WriteHeader(404)
			_ = json.NewEncoder(w).Encode(api.Error{Code: 404, Message: "Group not found"})
			return
		}
	}

	// generates a random code for the sign-up link (just a random string of 32 characters)
	code, err := generateRandomCode(32)
	if err != nil {
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 500, Message: "Failed to generate sign-up code"})
		return
	}

	// this makes it so that if the scopeID is uuid.Nil, it will be nil in the database (instead of 0000000-0000-0000-0000-000000000000)
	var scopeID *uuid.UUID
	if req.ScopeID != uuid.Nil {
		scopeID = &req.ScopeID
	} else {
		scopeID = nil
	}

	params := db.CreateSignUpCodeParams{
		Code:      code,
		Email:     string(req.Email),
		RoleName:  req.RoleName,
		Scope:     db.ScopeType(req.Scope),
		ScopeID:   scopeID,
		CreatedBy: user.ID,
	}

	signupCode, err := s.db.Queries().CreateSignUpCode(ctx, params)
	if err != nil {
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 500, Message: "An unexpected error occurred."})
		return
	}

	w.WriteHeader(201)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": signupCode.Code})
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
