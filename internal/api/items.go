package api

import (
	"context"
	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/jackc/pgx/v5/pgtype"
	"log"
)

func (s Server) GetItems(ctx context.Context, request api.GetItemsRequestObject) (api.GetItemsResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetItems401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_items", nil)
	if err != nil {
		log.Printf("Error checking view_items permission: %v", err)
		return api.GetItems500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetItems403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	items, err := s.db.Queries().GetAllItems(ctx)
	if err != nil {
		log.Printf("Failed to get items: %v", err)
		return api.GetItems500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred.",
		}, nil
	}

	// Convert database items to API response format
	var response api.GetItems200JSONResponse
	for _, item := range items {
		id := item.ID
		name := item.Name
		description := item.Description.String
		itemType := api.ItemType(item.Type)
		stock := int(item.Stock)
		urls := item.Urls

		itemResponse := api.ItemResponse{
			Id:          id,
			Name:        name,
			Description: &description,
			Type:        itemType,
			Stock:       stock,
			Urls:        &urls,
		}
		response = append(response, itemResponse)
	}

	return response, nil
}

func (s Server) GetItemsByType(ctx context.Context, request api.GetItemsByTypeRequestObject) (api.GetItemsByTypeResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetItemsByType401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_items", nil)
	if err != nil {
		log.Printf("Error checking view_items permission: %v", err)
		return api.GetItemsByType500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetItemsByType403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	items, err := s.db.Queries().GetItemsByType(ctx, db.ItemType(request.Type))
	if err != nil {
		log.Printf("Failed to get items by type: %v", err)
		return api.GetItemsByType500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred.",
		}, nil
	}

	// Convert database items to API response format
	var response api.GetItemsByType200JSONResponse
	for _, item := range items {
		id := item.ID
		name := item.Name
		description := item.Description.String
		itemType := api.ItemType(item.Type)
		stock := int(item.Stock)
		urls := item.Urls

		itemResponse := api.ItemResponse{
			Id:          id,
			Name:        name,
			Description: &description,
			Type:        itemType,
			Stock:       stock,
			Urls:        &urls,
		}
		response = append(response, itemResponse)
	}

	return response, nil
}

func (s Server) GetItemById(ctx context.Context, request api.GetItemByIdRequestObject) (api.GetItemByIdResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetItemById401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_items", nil)
	if err != nil {
		log.Printf("Error checking view_items permission: %v", err)
		return api.GetItemById500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetItemById403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	item, err := s.db.Queries().GetItemByID(ctx, request.Id)
	if err != nil {
		return api.GetItemById404JSONResponse{
			Code:    404,
			Message: "Item not found",
		}, nil
	}

	id := item.ID
	name := item.Name
	description := item.Description.String
	itemType := api.ItemType(item.Type)
	stock := int(item.Stock)
	urls := item.Urls

	return api.GetItemById200JSONResponse{
		Id:          id,
		Name:        name,
		Description: &description,
		Type:        itemType,
		Stock:       stock,
		Urls:        &urls,
	}, nil
}

func (s Server) CreateItem(ctx context.Context, request api.CreateItemRequestObject) (api.CreateItemResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.CreateItem401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_items", nil)
	if err != nil {
		log.Printf("Error checking manage_items permission: %v", err)
		return api.CreateItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.CreateItem403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	if request.Body == nil {
		return api.CreateItem400JSONResponse{
			Code:    400,
			Message: "Request body is required",
		}, nil
	}

	req := *request.Body

	var urls []string
	if req.Urls != nil {
		urls = *req.Urls
	} else {
		urls = []string{}
	}

	params := db.CreateItemParams{
		Name:        req.Name,
		Description: pgtype.Text{String: "", Valid: false},
		Type:        db.ItemType(req.Type),
		Stock:       int32(req.Stock),
		Urls:        urls,
	}

	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}

	item, err := s.db.Queries().CreateItem(ctx, params)
	if err != nil {
		log.Printf("Failed to create item: %v", err)
		return api.CreateItem500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred.",
		}, nil
	}

	id := item.ID
	name := item.Name
	description := item.Description.String
	itemType := api.ItemType(item.Type)
	stock := int(item.Stock)

	return api.CreateItem201JSONResponse{
		Id:          id,
		Name:        name,
		Description: &description,
		Type:        itemType,
		Stock:       stock,
		Urls:        &urls,
	}, nil
}

func (s Server) UpdateItem(ctx context.Context, request api.UpdateItemRequestObject) (api.UpdateItemResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.UpdateItem401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_items", nil)
	if err != nil {
		log.Printf("Error checking manage_items permission: %v", err)
		return api.UpdateItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.UpdateItem403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	if request.Body == nil {
		return api.UpdateItem400JSONResponse{
			Code:    400,
			Message: "Request body is required",
		}, nil
	}

	req := *request.Body

	var urls []string
	if req.Urls != nil {
		urls = *req.Urls
	} else {
		urls = []string{}
	}

	params := db.UpdateItemParams{
		ID:          request.Id,
		Name:        req.Name,
		Description: pgtype.Text{String: "", Valid: false},
		Type:        db.ItemType(req.Type),
		Stock:       int32(req.Stock),
		Urls:        urls,
	}

	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}

	item, err := s.db.Queries().UpdateItem(ctx, params)
	if err != nil {
		log.Printf("Failed to update item: %v", err)
		return api.UpdateItem404JSONResponse{
			Code:    404,
			Message: "Item not found",
		}, nil
	}

	id := item.ID
	name := item.Name
	description := item.Description.String
	itemType := api.ItemType(item.Type)
	stock := int(item.Stock)

	return api.UpdateItem200JSONResponse{
		Id:          id,
		Name:        name,
		Description: &description,
		Type:        itemType,
		Stock:       stock,
		Urls:        &urls,
	}, nil
}

func (s Server) PatchItem(ctx context.Context, request api.PatchItemRequestObject) (api.PatchItemResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.PatchItem401JSONResponse{Code: 401, Message: "Unauthorized"}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_items", nil)
	if err != nil {
		log.Printf("Error checking manage_items permission: %v", err)
		return api.PatchItem500JSONResponse{Code: 500, Message: "Internal server error"}, nil
	}
	if !hasPermission {
		return api.PatchItem403JSONResponse{Code: 403, Message: "Insufficient permissions"}, nil
	}

	if request.Body == nil {
		return api.PatchItem400JSONResponse{Code: 400, Message: "Request body is required"}, nil
	}

	req := *request.Body

	params := db.PatchItemParams{
		ID: request.Id,
	}

	if req.Name != "" {
		params.Name = pgtype.Text{String: req.Name, Valid: true}
	}

	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}

	if req.Type != "" {
		params.Type = db.NullItemType{ItemType: db.ItemType(req.Type), Valid: true}
	}

	if req.Stock != 0 {
		params.Stock = pgtype.Int4{Int32: int32(req.Stock), Valid: true}
	}

	if req.Urls != nil {
		params.Urls = *req.Urls
	}

	item, err := s.db.Queries().PatchItem(ctx, params)

	if err != nil {
		return api.PatchItem404JSONResponse{Code: 404, Message: "Item not found"}, nil
	}

	id := item.ID
	name := item.Name
	description := item.Description.String
	itemType := api.ItemType(item.Type)
	stock := int(item.Stock)
	urls := item.Urls

	return api.PatchItem200JSONResponse{
		Id:          id,
		Name:        name,
		Description: &description,
		Type:        itemType,
		Stock:       stock,
		Urls:        &urls,
	}, nil
}

func (s Server) DeleteItem(ctx context.Context, request api.DeleteItemRequestObject) (api.DeleteItemResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.DeleteItem401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_items", nil)
	if err != nil {
		log.Printf("Error checking manage_items permission: %v", err)
		return api.DeleteItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.DeleteItem403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	err = s.db.Queries().DeleteItem(ctx, request.Id)
	if err != nil {
		log.Printf("Failed to delete item: %v", err)
		return api.DeleteItem404JSONResponse{
			Code:    404,
			Message: "Item not found",
		}, nil
	}

	return api.DeleteItem204Response{}, nil
}
