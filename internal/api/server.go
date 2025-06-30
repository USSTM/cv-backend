package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/database"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	db            *database.Database
	jwtService    *auth.JWTService
	authenticator *auth.Authenticator
}

func NewServer(db *database.Database, jwtService *auth.JWTService, authenticator *auth.Authenticator) *Server {
	return &Server{
		db:            db,
		jwtService:    jwtService,
		authenticator: authenticator,
	}
}

func (s Server) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req api.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	user, err := s.db.Queries().GetUserByEmail(ctx, string(req.Email))
	if err != nil {
		log.Printf("User not found: %v", err)
		resp := api.Error{
			Code:    400,
			Message: "Invalid email or password.",
		}
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// TODO: Add password field to LoginRequest schema
	// For now, validate against a hardcoded password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("password")); err != nil {
		log.Printf("Invalid password: %v", err)
		resp := api.Error{
			Code:    400,
			Message: "Invalid email or password.",
		}
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	userUUID := user.ID

	token, err := s.jwtService.GenerateToken(ctx, userUUID)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		resp := api.Error{
			Code:    500,
			Message: "An unexpected error occurred.",
		}
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	log.Printf("User logged in: %s", user.Email)
	resp := api.LoginResponse{
		Token: &token,
	}

	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s Server) PingProtected(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		resp := api.Error{
			Code:    401,
			Message: "Unauthorized!",
		}
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_own_data", nil)
	if err != nil {
		log.Printf("Error checking view_own_data permission: %v", err)
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

	resp := api.PingResponse{
		Message:   "PONG! Hello " + user.Email,
		Timestamp: time.Now(),
	}

	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s Server) GetItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check permission
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		resp := api.Error{Code: 401, Message: "Unauthorized"}
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_items", nil)
	if err != nil {
		log.Printf("Error checking view_items permission: %v", err)
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

	items, err := s.db.Queries().GetAllItems(ctx)
	if err != nil {
		log.Printf("Failed to get items: %v", err)
		resp := api.Error{
			Code:    500,
			Message: "An unexpected error occurred.",
		}
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Convert database items to API response format
	var response []any
	for _, item := range items {
		itemResponse := map[string]any{
			"id":          item.ID.String(),
			"name":        item.Name,
			"description": item.Description.String,
			"type":        string(item.Type),
			"stock":       item.Stock,
		}
		response = append(response, itemResponse)
	}

	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(response)
}

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
