package api

import (
	"context"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s Server) GetAllGroups(ctx context.Context, request api.GetAllGroupsRequestObject) (api.GetAllGroupsResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetAllGroups401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewGroupData, nil)
	if err != nil {
		logger.Error("Error checking view_group_data permission",
			"user_id", user.ID,
			"permission", rbac.ViewGroupData,
			"error", err)
		return api.GetAllGroups500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetAllGroups403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	groups, err := s.db.Queries().GetAllGroups(ctx)
	if err != nil {
		return api.GetAllGroups500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	var response api.GetAllGroups200JSONResponse
	for _, group := range groups {
		var description *string
		if group.Description.Valid {
			description = &group.Description.String
		}
		logoURL, thumbURL := s.resolveGroupLogoURLs(ctx, group)
		response = append(response, api.Group{
			Id:               group.ID,
			Name:             group.Name,
			Description:      description,
			LogoUrl:          logoURL,
			LogoThumbnailUrl: thumbURL,
		})
	}

	return response, nil
}

func (s Server) GetGroupByID(ctx context.Context, request api.GetGroupByIDRequestObject) (api.GetGroupByIDResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetGroupByID401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewGroupData, nil)
	if err != nil {
		logger.Error("Error checking view_group_data permission",
			"user_id", user.ID,
			"permission", rbac.ViewGroupData,
			"error", err)
		return api.GetGroupByID500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetGroupByID403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	group, err := s.db.Queries().GetGroupByID(ctx, request.Id)
	if err != nil {
		return api.GetGroupByID404JSONResponse(NotFound("Group").Create()), nil
	}

	var description *string
	if group.Description.Valid {
		description = &group.Description.String
	}
	logoURL, thumbURL := s.resolveGroupLogoURLs(ctx, group)
	response := api.GetGroupByID200JSONResponse{
		Id:               group.ID,
		Name:             group.Name,
		Description:      description,
		LogoUrl:          logoURL,
		LogoThumbnailUrl: thumbURL,
	}

	return response, nil
}

func (s Server) CreateGroup(ctx context.Context, request api.CreateGroupRequestObject) (api.CreateGroupResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.CreateGroup401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageGroups, nil)
	if err != nil {
		logger.Error("Error checking manage_groups permission",
			"user_id", user.ID,
			"permission", rbac.ManageGroups,
			"error", err)
		return api.CreateGroup500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.CreateGroup403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	groupParams := db.CreateGroupParams{
		Name:        request.Body.Name,
		Description: pgtype.Text{String: *request.Body.Description, Valid: request.Body.Description != nil},
	}

	group, err := s.db.Queries().CreateGroup(ctx, groupParams)
	if err != nil {
		logger.Error("Failed to create group",
			"group_name", request.Body.Name,
			"error", err)
		return api.CreateGroup500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	var description *string
	if group.Description.Valid {
		description = &group.Description.String
	}
	response := api.CreateGroup201JSONResponse{
		Id:          group.ID,
		Name:        group.Name,
		Description: description,
	}

	return response, nil
}

func (s Server) UpdateGroup(ctx context.Context, request api.UpdateGroupRequestObject) (api.UpdateGroupResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.UpdateGroup401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageGroups, nil)
	if err != nil {
		logger.Error("Error checking manage_groups permission",
			"user_id", user.ID,
			"permission", rbac.ManageGroups,
			"error", err)
		return api.UpdateGroup500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.UpdateGroup403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	groupParams := db.UpdateGroupParams{
		ID:          request.Id,
		Name:        request.Body.Name,
		Description: pgtype.Text{String: *request.Body.Description, Valid: request.Body.Description != nil},
	}

	group, err := s.db.Queries().UpdateGroup(ctx, groupParams)
	if err != nil {
		logger.Error("Failed to update group",
			"group_id", request.Id,
			"error", err)
		return api.UpdateGroup500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
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

func (s Server) resolveGroupLogoURLs(ctx context.Context, g db.Group) (logoURL, thumbnailURL *string) {
	if !g.LogoS3Key.Valid {
		return nil, nil
	}
	url, err := s.s3Service.GeneratePresignedURL(ctx, "GET", g.LogoS3Key.String, time.Hour)
	if err != nil {
		return nil, nil
	}
	var thumbURL *string
	if g.LogoThumbnailS3Key.Valid {
		t, err := s.s3Service.GeneratePresignedURL(ctx, "GET", g.LogoThumbnailS3Key.String, time.Hour)
		if err == nil {
			thumbURL = &t
		}
	}
	return &url, thumbURL
}

func (s Server) DeleteGroup(ctx context.Context, request api.DeleteGroupRequestObject) (api.DeleteGroupResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.DeleteGroup401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageGroups, nil)
	if err != nil {
		logger.Error("Error checking manage_groups permission",
			"user_id", user.ID,
			"permission", rbac.ManageGroups,
			"error", err)
		return api.DeleteGroup500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.DeleteGroup403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	group, err := s.db.Queries().GetGroupByID(ctx, request.Id)
	if err != nil {
		return api.DeleteGroup404JSONResponse(NotFound("Group").Create()), nil
	}

	if group.LogoS3Key.Valid {
		_ = s.s3Service.DeleteObject(ctx, group.LogoS3Key.String)
	}
	if group.LogoThumbnailS3Key.Valid {
		_ = s.s3Service.DeleteObject(ctx, group.LogoThumbnailS3Key.String)
	}

	err = s.db.Queries().DeleteGroup(ctx, request.Id)
	if err != nil {
		logger.Error("Failed to delete group",
			"group_id", request.Id,
			"error", err)
		return api.DeleteGroup500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	return api.DeleteGroup204Response{}, nil
}
