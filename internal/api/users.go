package api

import (
	"context"
	"log"

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
