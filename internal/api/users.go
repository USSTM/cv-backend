package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s Server) GetUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check permission
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		resp := api.Error{Code: 401, Message: "Unauthorized"}
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_users", nil)
	if err != nil {
		log.Printf("Error checking manage_users permission: %v", err)
		resp := api.Error{Code: 500, Message: "Internal server error"}
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}
	if !hasPermission {
		resp := api.Error{Code: 403, Message: "Insufficient permissions"}
		w.WriteHeader(403)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	users, err := s.db.Queries().GetAllUsers(ctx)
	if err != nil {
		log.Printf("Failed to get users: %v", err)
		resp := api.Error{
			Code:    500,
			Message: "An unexpected error occurred.",
		}
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Convert database users to API response format
	var response []api.User
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
			Email: openapi_types.Email(user.Email),
			Role:  role,
		}
		response = append(response, userResponse)
	}

	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(response)
}