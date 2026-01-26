package api

import (
	"context"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/rbac"
)

func (s Server) GetNotifications(ctx context.Context, request api.GetNotificationsRequestObject) (api.GetNotificationsResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetNotifications401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewNotifications, nil)
	if err != nil {
		logger.Error("Failed to check permissions", "error", err, "user_id", user.ID)
		return api.GetNotifications500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	if !hasPermission {
		return api.GetNotifications403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// Stub: Return empty list
	var notifications []api.NotificationResponse = []api.NotificationResponse{}
	return api.GetNotifications200JSONResponse(notifications), nil
}

func (s Server) GetNotificationStats(ctx context.Context, request api.GetNotificationStatsRequestObject) (api.GetNotificationStatsResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetNotificationStats401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewNotifications, nil)
	if err != nil {
		logger.Error("Failed to check permissions", "error", err, "user_id", user.ID)
		return api.GetNotificationStats500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	if !hasPermission {
		return api.GetNotificationStats403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// Stub: Return zero stats
	return api.GetNotificationStats200JSONResponse{
		TotalNotifications:      0,
		UnreadCount:             0,
		HighPriorityCount:       0,
		UnreadHighPriorityCount: 0,
	}, nil
}

func (s Server) MarkNotificationsAsRead(ctx context.Context, request api.MarkNotificationsAsReadRequestObject) (api.MarkNotificationsAsReadResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.MarkNotificationsAsRead401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageNotifications, nil)
	if err != nil {
		logger.Error("Failed to check permissions", "error", err, "user_id", user.ID)
		return api.MarkNotificationsAsRead500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	if !hasPermission {
		return api.MarkNotificationsAsRead403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	if request.Body == nil {
		return api.MarkNotificationsAsRead400JSONResponse{
			Code:    400,
			Message: "Request body is required",
		}, nil
	}

	// Stub: Return 0 marked
	markedCountInt := 0
	return api.MarkNotificationsAsRead200JSONResponse{
		MarkedCount: &markedCountInt,
	}, nil
}

func (s Server) GetNotification(ctx context.Context, request api.GetNotificationRequestObject) (api.GetNotificationResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetNotification401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewNotifications, nil)
	if err != nil {
		logger.Error("Failed to check permissions", "error", err, "user_id", user.ID)
		return api.GetNotification500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	if !hasPermission {
		return api.GetNotification403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// Stub: Return 404 as we have no notifications
	return api.GetNotification404JSONResponse{
		Code:    404,
		Message: "Notification not found",
	}, nil
}

func (s Server) DeleteNotification(ctx context.Context, request api.DeleteNotificationRequestObject) (api.DeleteNotificationResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.DeleteNotification401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageNotifications, nil)
	if err != nil {
		logger.Error("Failed to check permissions", "error", err, "user_id", user.ID)
		return api.DeleteNotification500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	if !hasPermission {
		return api.DeleteNotification403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// Stub: Return success (idempotent)
	return api.DeleteNotification204Response{}, nil
}

func (s Server) CreateNotification(ctx context.Context, request api.CreateNotificationRequestObject) (api.CreateNotificationResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.CreateNotification401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageNotifications, nil)
	if err != nil {
		logger.Error("Failed to check permissions", "error", err, "user_id", user.ID)
		return api.CreateNotification500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	if !hasPermission {
		return api.CreateNotification403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	if request.Body == nil {
		return api.CreateNotification400JSONResponse{
			Code:    400,
			Message: "Request body is required",
		}, nil
	}

	// Stub: Return success with dummy data
	// We can't really return a valid object effectively without more complex mocking,
	// but a 201 with an empty/mock object satisfies the contract if the client handles it.
	// Or we can return a 501 Not Implemented if preferred, but user asked for "empty response"
	// For Create, a successful mock is likely what they want to not break frontends found in other flows.

	// Constructing a minimal dummy response
	apiNotification := api.NotificationResponse{
		Id:      user.ID, // Just using a UUID
		UserId:  user.ID,
		Title:   request.Body.Title,
		Message: request.Body.Message,
	}

	return api.CreateNotification201JSONResponse(apiNotification), nil
}
