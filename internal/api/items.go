package api

import (
	"encoding/json"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/jackc/pgx/v5/pgtype"
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

func (s Server) GetItemsByType(w http.ResponseWriter, r *http.Request, itemType api.ItemType) {
	ctx := r.Context()
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 401, Message: "Unauthorized"})
		return
	}
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_items", nil)
	if err != nil || !hasPermission {
		w.WriteHeader(403)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 403, Message: "Insufficient permissions"})
		return
	}
	items, err := s.db.Queries().GetItemsByType(ctx, db.ItemType(itemType))
	if err != nil {
		log.Printf("Failed to get items by type: %v", err)
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 500, Message: "An unexpected error occurred."})
		return
	}
	var response []any
	for _, item := range items {
		response = append(response, map[string]any{
			"id":          item.ID.String(),
			"name":        item.Name,
			"description": item.Description.String,
			"type":        string(item.Type),
			"stock":       item.Stock,
		})
	}
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(response)
}

func (s Server) GetItemById(w http.ResponseWriter, r *http.Request, id api.UUID) {
	ctx := r.Context()
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 401, Message: "Unauthorized"})
		return
	}
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_items", nil)
	if err != nil || !hasPermission {
		w.WriteHeader(403)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 403, Message: "Insufficient permissions"})
		return
	}
	item, err := s.db.Queries().GetItemByID(ctx, id)
	if err != nil {
		w.WriteHeader(404)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 404, Message: "Item not found"})
		return
	}
	response := map[string]any{
		"id":          item.ID.String(),
		"name":        item.Name,
		"description": item.Description.String,
		"type":        string(item.Type),
		"stock":       item.Stock,
	}
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(response)
}

func (s Server) CreateItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 401, Message: "Unauthorized"})
		return
	}
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_items", nil)
	if err != nil || !hasPermission {
		w.WriteHeader(403)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 403, Message: "Insufficient permissions"})
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Stock       int32  `json:"stock"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 400, Message: "Invalid request body"})
		return
	}
	params := db.CreateItemParams{
		Name:        req.Name,
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Type:        db.ItemType(req.Type),
		Stock:       req.Stock,
	}
	item, err := s.db.Queries().CreateItem(ctx, params)
	if err != nil {
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 500, Message: "An unexpected error occurred."})
		return
	}
	response := map[string]any{
		"id":          item.ID.String(),
		"name":        item.Name,
		"description": item.Description.String,
		"type":        string(item.Type),
		"stock":       item.Stock,
	}
	w.WriteHeader(201)
	_ = json.NewEncoder(w).Encode(response)
}

func (s Server) UpdateItem(w http.ResponseWriter, r *http.Request, id api.UUID) {
	ctx := r.Context()
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 401, Message: "Unauthorized"})
		return
	}
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_items", nil)
	if err != nil || !hasPermission {
		w.WriteHeader(403)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 403, Message: "Insufficient permissions"})
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Stock       int32  `json:"stock"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 400, Message: "Invalid request body"})
		return
	}
	params := db.UpdateItemParams{
		ID:          id,
		Name:        req.Name,
		Description: pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Type:        db.ItemType(req.Type),
		Stock:       req.Stock,
	}
	item, err := s.db.Queries().UpdateItem(ctx, params)
	if err != nil {
		w.WriteHeader(404)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 404, Message: "Item not found"})
		return
	}
	response := map[string]any{
		"id":          item.ID.String(),
		"name":        item.Name,
		"description": item.Description.String,
		"type":        string(item.Type),
		"stock":       item.Stock,
	}
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(response)
}

func (s Server) DeleteItem(w http.ResponseWriter, r *http.Request, id api.UUID) {
	ctx := r.Context()
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 401, Message: "Unauthorized"})
		return
	}
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_items", nil)
	if err != nil || !hasPermission {
		w.WriteHeader(403)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 403, Message: "Insufficient permissions"})
		return
	}
	err = s.db.Queries().DeleteItem(ctx, id)
	if err != nil {
		w.WriteHeader(404)
		_ = json.NewEncoder(w).Encode(api.Error{Code: 404, Message: "Item not found"})
		return
	}
	w.WriteHeader(204)
}
