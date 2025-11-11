package api

import (
	"context"
	"log"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s Server) GetAllGroups(ctx context.Context, request api.GetAllGroupsRequestObject) (api.GetAllGroupsResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetAllGroups401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_group_data", nil)
	if err != nil {
		log.Printf("Error checking view_group_data permission: %v", err)
		return api.GetAllGroups500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetAllGroups403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	groups, err := s.db.Queries().GetAllGroups(ctx)
	if err != nil {
		return api.GetAllGroups500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred.",
		}, nil
	}

	var response api.GetAllGroups200JSONResponse
	for _, group := range groups {
		response = append(response, api.Group{
			Id:          group.ID,
			Name:        group.Name,
			Description: &group.Description.String,
		})
	}

	return response, nil
}

func (s Server) GetGroupByID(ctx context.Context, request api.GetGroupByIDRequestObject) (api.GetGroupByIDResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetGroupByID401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_group_data", nil)
	if err != nil {
		log.Printf("Error checking view_group_data permission: %v", err)
		return api.GetGroupByID500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetGroupByID403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	group, err := s.db.Queries().GetGroupByID(ctx, request.Id)
	if err != nil {
		return api.GetGroupByID404JSONResponse{
			Code:    404,
			Message: "Group not found",
		}, nil
	}

	response := api.GetGroupByID200JSONResponse{
		Id:          group.ID,
		Name:        group.Name,
		Description: &group.Description.String,
	}

	return response, nil
}

func (s Server) CreateGroup(ctx context.Context, request api.CreateGroupRequestObject) (api.CreateGroupResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.CreateGroup401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_groups", nil)
	if err != nil {
		log.Printf("Error checking manage_groups permission: %v", err)
		return api.CreateGroup500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.CreateGroup403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	groupParams := db.CreateGroupParams{
		Name:        request.Body.Name,
		Description: pgtype.Text{String: *request.Body.Description, Valid: request.Body.Description != nil},
	}

	group, err := s.db.Queries().CreateGroup(ctx, groupParams)
	if err != nil {
		log.Printf("Failed to create group: %v", err)
		return api.CreateGroup500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred.",
		}, nil
	}

	response := api.CreateGroup201JSONResponse{
		Id:          group.ID,
		Name:        group.Name,
		Description: &group.Description.String,
	}

	return response, nil
}

func (s Server) UpdateGroup(ctx context.Context, request api.UpdateGroupRequestObject) (api.UpdateGroupResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.UpdateGroup401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_groups", nil)
	if err != nil {
		log.Printf("Error checking manage_groups permission: %v", err)
		return api.UpdateGroup500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.UpdateGroup403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	groupParams := db.UpdateGroupParams{
		ID:          request.Id,
		Name:        request.Body.Name,
		Description: pgtype.Text{String: *request.Body.Description, Valid: request.Body.Description != nil},
	}

	group, err := s.db.Queries().UpdateGroup(ctx, groupParams)
	if err != nil {
		log.Printf("Failed to update group: %v", err)
		return api.UpdateGroup500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred.",
		}, nil
	}

	var description *string
	if group.Description.Valid {
		description = &group.Description.String
	}
	response := api.UpdateGroup200JSONResponse{
		Id:          group.ID,
		Name:        group.Name,
		Description: description,
	}

	return response, nil
}

func (s Server) DeleteGroup(ctx context.Context, request api.DeleteGroupRequestObject) (api.DeleteGroupResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.DeleteGroup401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "manage_groups", nil)
	if err != nil {
		log.Printf("Error checking manage_groups permission: %v", err)
		return api.DeleteGroup500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.DeleteGroup403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	err = s.db.Queries().DeleteGroup(ctx, request.Id)
	if err != nil {
		log.Printf("Failed to delete group: %v", err)
		return api.DeleteGroup500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred.",
		}, nil
	}

	return api.DeleteGroup204Response{}, nil
}
