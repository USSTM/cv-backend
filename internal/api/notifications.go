package api

import (
	"context"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s Server) GetNotifications(ctx context.Context, request api.GetNotificationsRequestObject) (api.GetNotificationsResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetNotifications401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)

	notifs, err := s.notificationService.GetUserNotifications(ctx, user.ID, limit, offset)
	if err != nil {
		logger.Error("Failed to get notifications", "error", err, "user_id", user.ID)
		return api.GetNotifications500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	var response []api.NotificationResponse
	for _, n := range notifs {
		response = append(response, api.NotificationResponse{
			NotificationId:        n.NotificationID,
			IsRead:                n.IsRead,
			NotificationCreatedAt: n.NotificationCreatedAt.Time,
			NotificationObjectId:  n.NotificationObjectID,
			EntityId:              n.EntityID,
			EntityTypeName:        n.EntityTypeName,
			ActorId:               n.ActorID,
			ActorEmail:            openapi_types.Email(n.ActorEmail),
		})
	}

	if response == nil {
		response = []api.NotificationResponse{}
	}

	total, err := s.notificationService.GetTotalCount(ctx, user.ID)
	if err != nil {
		logger.Error("Failed to get total notification count", "error", err, "user_id", user.ID)
		return api.GetNotifications500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	return api.GetNotifications200JSONResponse{
		Data: response,
		Meta: buildPaginationMeta(total, limit, offset),
	}, nil
}

func (s Server) GetUnreadNotificationCount(ctx context.Context, request api.GetUnreadNotificationCountRequestObject) (api.GetUnreadNotificationCountResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUnreadNotificationCount401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	count, err := s.notificationService.GetUnreadCount(ctx, user.ID)
	if err != nil {
		logger.Error("Failed to get unread notification count", "error", err, "user_id", user.ID)
		return api.GetUnreadNotificationCount500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	return api.GetUnreadNotificationCount200JSONResponse{
		UnreadCount: int(count),
	}, nil
}

func (s Server) MarkNotificationAsRead(ctx context.Context, request api.MarkNotificationAsReadRequestObject) (api.MarkNotificationAsReadResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.MarkNotificationAsRead401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	marked, err := s.notificationService.MarkAsRead(ctx, user.ID, request.Id)
	if err != nil {
		logger.Error("Failed to mark notification as read", "error", err, "user_id", user.ID, "notif_id", request.Id)
		return api.MarkNotificationAsRead404JSONResponse(NotFound("Notification").Create()), nil
	}

	return api.MarkNotificationAsRead200JSONResponse{
		NotificationId:        marked.ID,
		IsRead:                marked.IsRead,
		NotificationCreatedAt: marked.CreatedAt.Time,
		NotificationObjectId:  marked.NotificationObjectID,
		EntityId:              marked.ID,
		EntityTypeName:        "",
		ActorId:               marked.NotifierID,
		ActorEmail:            "",
	}, nil
}

func (s Server) MarkAllNotificationsAsRead(ctx context.Context, request api.MarkAllNotificationsAsReadRequestObject) (api.MarkAllNotificationsAsReadResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.MarkAllNotificationsAsRead401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	err := s.notificationService.MarkAllAsRead(ctx, user.ID)
	if err != nil {
		logger.Error("Failed to mark all notifications as read", "error", err, "user_id", user.ID)
		return api.MarkAllNotificationsAsRead500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	return api.MarkAllNotificationsAsRead200JSONResponse{
		Message: "Success",
	}, nil
}
