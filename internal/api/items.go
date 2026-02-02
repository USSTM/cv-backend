package api

import (
	"context"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s Server) GetItems(ctx context.Context, request api.GetItemsRequestObject) (api.GetItemsResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetItems401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewItems, nil)
	if err != nil {
		logger.Error("Error checking rbac.ViewItems permission", "error", err)
		return api.GetItems500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetItems403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)

	items, err := s.db.Queries().GetAllItems(ctx, db.GetAllItemsParams{Limit: limit, Offset: offset})
	if err != nil {
		logger.Error("Failed to get items", "error", err)
		return api.GetItems500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	total, err := s.db.Queries().CountAllItems(ctx)
	if err != nil {
		logger.Error("Failed to count items", "error", err)
		return api.GetItems500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	// Convert database items to API response format
	var response []api.ItemResponse
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

	if response == nil {
		response = []api.ItemResponse{}
	}

	return api.GetItems200JSONResponse{
		Data: response,
		Meta: api.PaginationMeta{
			Total:   int(total),
			Limit:   int(limit),
			Offset:  int(offset),
			HasMore: int(offset)+int(limit) < int(total),
		},
	}, nil
}

func (s Server) GetItemsByType(ctx context.Context, request api.GetItemsByTypeRequestObject) (api.GetItemsByTypeResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetItemsByType401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewItems, nil)
	if err != nil {
		logger.Error("Error checking rbac.ViewItems permission", "error", err)
		return api.GetItemsByType500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetItemsByType403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)

	items, err := s.db.Queries().GetItemsByType(ctx, db.GetItemsByTypeParams{
		Type:   db.ItemType(request.Type),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		logger.Error("Failed to get items by type", "error", err)
		return api.GetItemsByType500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	total, err := s.db.Queries().CountItemsByType(ctx, db.ItemType(request.Type))
	if err != nil {
		logger.Error("Failed to count items by type", "error", err)
		return api.GetItemsByType500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	// Convert database items to API response format
	var response []api.ItemResponse
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

	if response == nil {
		response = []api.ItemResponse{}
	}

	return api.GetItemsByType200JSONResponse{
		Data: response,
		Meta: api.PaginationMeta{
			Total:   int(total),
			Limit:   int(limit),
			Offset:  int(offset),
			HasMore: int(offset)+int(limit) < int(total),
		},
	}, nil
}

func (s Server) GetItemById(ctx context.Context, request api.GetItemByIdRequestObject) (api.GetItemByIdResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetItemById401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewItems, nil)
	if err != nil {
		logger.Error("Error checking rbac.ViewItems permission", "error", err)
		return api.GetItemById500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetItemById403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	item, err := s.db.Queries().GetItemByID(ctx, request.Id)
	if err != nil {
		return api.GetItemById404JSONResponse(NotFound("Item").Create()), nil
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
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.CreateItem401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageItems, nil)
	if err != nil {
		logger.Error("Error checking rbac.ManageItems permission", "error", err)
		return api.CreateItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.CreateItem403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	if request.Body == nil {
		return api.CreateItem400JSONResponse(ValidationErr("Request body is required", nil).Create()), nil
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
		logger.Error("Failed to create item", "error", err)
		return api.CreateItem500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
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
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.UpdateItem401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageItems, nil)
	if err != nil {
		logger.Error("Error checking rbac.ManageItems permission", "error", err)
		return api.UpdateItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.UpdateItem403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	if request.Body == nil {
		return api.UpdateItem400JSONResponse(ValidationErr("Request body is required", nil).Create()), nil
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
		logger.Error("Failed to update item", "error", err)
		return api.UpdateItem404JSONResponse(NotFound("Item").Create()), nil
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
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.PatchItem401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageItems, nil)
	if err != nil {
		logger.Error("Error checking rbac.ManageItems permission", "error", err)
		return api.PatchItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.PatchItem403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	if request.Body == nil {
		return api.PatchItem400JSONResponse(ValidationErr("Request body is required", nil).Create()), nil
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
		return api.PatchItem404JSONResponse(NotFound("Item").Create()), nil
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
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.DeleteItem401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageItems, nil)
	if err != nil {
		logger.Error("Error checking rbac.ManageItems permission", "error", err)
		return api.DeleteItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.DeleteItem403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	err = s.db.Queries().DeleteItem(ctx, request.Id)
	if err != nil {
		logger.Error("Failed to delete item", "error", err)
		return api.DeleteItem404JSONResponse(NotFound("Item").Create()), nil
	}

	return api.DeleteItem204Response{}, nil
}
