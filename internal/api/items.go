package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
)

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