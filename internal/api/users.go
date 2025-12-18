package api

import (
	"github.com/USSTM/cv-backend/internal/rbac"
	"context"
	"crypto/rand"
	"strings"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

func (s Server) GetUsers(ctx context.Context, request api.GetUsersRequestObject) (api.GetUsersResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	// Check permission
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUsers401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageUsers, nil)
	if err != nil {
		logger.Error("Error checking manage_users permission",
			"user_id", user.ID,
			"permission", rbac.ManageUsers,
			"error", err)
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
		logger.Error("Failed to get users", "error", err)
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
			logger.Error("Failed to get user roles",
				"user_id", user.ID,
				"error", err)
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
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageGroupUsers, request.Body.ScopeId)
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
		scopeID = *req.ScopeId
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

func (s Server) GetUsersByGroup(ctx context.Context, request api.GetUsersByGroupRequestObject) (api.GetUsersByGroupResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUsersByGroup401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageGroupUsers, &request.GroupId)
	if err != nil {
		logger.Error("Error checking manage_group_users permission", "error", err)
		return api.GetUsersByGroup500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetUsersByGroup403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	_, err = s.db.Queries().GetGroupByID(ctx, request.GroupId)
	if err != nil {
		return api.GetUsersByGroup404JSONResponse{
			Code:    404,
			Message: "Group not found",
		}, nil
	}

	users, err := s.db.Queries().GetUsersByGroup(ctx, &request.GroupId)
	if err != nil {
		logger.Error("Failed to get users by group", "error", err)
		return api.GetUsersByGroup500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred.",
		}, nil
	}

	if len(users) == 0 {
		return api.GetUsersByGroup404JSONResponse{
			Code:    404,
			Message: "No users found in the specified group",
		}, nil
	}

	var response api.GetUsersByGroup200JSONResponse
	for _, user := range users {
		groupUser := api.GroupUser{
			Id:       user.ID,
			Email:    types.Email(user.Email),
			RoleName: user.RoleName.String,
			Scope:    string(user.Scope),
		}

		if user.ScopeID != nil {
			groupUser.ScopeId = user.ScopeID
		}

		response = append(response, groupUser)
	}

	return response, nil
}

func (s Server) GetUserById(ctx context.Context, request api.GetUserByIdRequestObject) (api.GetUserByIdResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	// Check authentication
	currentUser, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUserById401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	// Users can view their own data, or admins can view any user
	canView := currentUser.ID == request.UserId
	if !canView {
		hasPermission, err := s.authenticator.CheckPermission(ctx, currentUser.ID, rbac.ManageUsers, nil)
		if err != nil {
			logger.Error("Error checking manage_users permission", "error", err)
			return api.GetUserById500JSONResponse{
				Code:    500,
				Message: "Internal server error",
			}, nil
		}
		canView = hasPermission
	}

	if !canView {
		return api.GetUserById403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	user, err := s.db.Queries().GetUserByID(ctx, request.UserId)
	if err != nil {
		return api.GetUserById404JSONResponse{
			Code:    404,
			Message: "User not found",
		}, nil
	}

	roles, err := s.db.Queries().GetUserRoles(ctx, &user.ID)
	if err != nil {
		logger.Error("Failed to get user roles", "error", err)
	}

	userResponse := api.User{
		Id:    user.ID,
		Email: types.Email(user.Email),
		Role:  GetUserRole(roles),
	}

	return api.GetUserById200JSONResponse(userResponse), nil
}

func (s Server) GetUserByEmail(ctx context.Context, request api.GetUserByEmailRequestObject) (api.GetUserByEmailResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUserByEmail401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageUsers, nil)
	if err != nil {
		logger.Error("Error checking manage_users permission", "error", err)
		return api.GetUserByEmail500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetUserByEmail403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	foundUser, err := s.db.Queries().GetUserByEmail(ctx, string(request.Email))
	if err != nil {
		return api.GetUserByEmail404JSONResponse{
			Code:    404,
			Message: "User not found",
		}, nil
	}

	roles, err := s.db.Queries().GetUserRoles(ctx, &foundUser.ID)
	if err != nil {
		logger.Error("Failed to get user roles", "error", err)
	}

	userResponse := api.User{
		Id:    foundUser.ID,
		Email: types.Email(foundUser.Email),
		Role:  GetUserRole(roles),
	}

	return api.GetUserByEmail200JSONResponse(userResponse), nil
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

func GetUserRole(roles []db.GetUserRolesRow) api.UserRole {
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

	return role
}
