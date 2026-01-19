package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func pgTimestampToTime(pgTime pgtype.Timestamp) *time.Time {
	if !pgTime.Valid {
		return nil
	}
	return &pgTime.Time
}

func timeToPgTimestamp(t *time.Time) pgtype.Timestamp {
	if t == nil {
		return pgtype.Timestamp{Valid: false}
	}
	return pgtype.Timestamp{Time: *t, Valid: true}
}

func convertNotificationToAPI(notification db.Notification) api.NotificationResponse {
	var metadata *map[string]interface{}
	if notification.Metadata != nil {
		var metaMap map[string]interface{}
		if err := json.Unmarshal(notification.Metadata, &metaMap); err == nil {
			metadata = &metaMap
		}
	}

	createdAt := notification.CreatedAt.Time

	return api.NotificationResponse{
		Id:               notification.ID,
		UserId:           notification.UserID,
		Type:             api.NotificationType(notification.Type),
		Title:            notification.Title,
		Message:          notification.Message,
		Priority:         api.NotificationPriority(notification.Priority),
		ReadAt:           pgTimestampToTime(notification.ReadAt),
		CreatedAt:        createdAt,
		ExpiresAt:        pgTimestampToTime(notification.ExpiresAt),
		RelatedBookingId: notification.RelatedBookingID,
		RelatedRequestId: notification.RelatedRequestID,
		RelatedItemId:    notification.RelatedItemID,
		RelatedUserId:    notification.RelatedUserID,
		Metadata:         metadata,
	}
}

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

	limit := int64(20)
	offset := int64(0)

	if request.Params.Limit != nil {
		limit = int64(*request.Params.Limit)
	}
	if request.Params.Offset != nil {
		offset = int64(*request.Params.Offset)
	}

	var result []db.Notification

	if request.Params.UnreadOnly != nil && *request.Params.UnreadOnly {
		result, err = s.db.Queries().GetUnreadUserNotifications(ctx, db.GetUnreadUserNotificationsParams{
			UserID: user.ID,
			Limit:  limit,
			Offset: offset,
		})
	} else if request.Params.Type != nil {
		result, err = s.db.Queries().GetNotificationsByType(ctx, db.GetNotificationsByTypeParams{
			UserID: user.ID,
			Type:   db.NotificationType(*request.Params.Type),
			Limit:  limit,
			Offset: offset,
		})
	} else if request.Params.Priority != nil {
		result, err = s.db.Queries().GetNotificationsByPriority(ctx, db.GetNotificationsByPriorityParams{
			UserID:   user.ID,
			Priority: db.NotificationPriority(*request.Params.Priority),
			Limit:    limit,
			Offset:   offset,
		})
	} else {
		result, err = s.db.Queries().GetUserNotifications(ctx, db.GetUserNotificationsParams{
			UserID: user.ID,
			Limit:  limit,
			Offset: offset,
		})
	}

	if err != nil {
		logger.Error("Failed to fetch notifications", "error", err, "user_id", user.ID)
		return api.GetNotifications500JSONResponse{
			Code:    500,
			Message: "Failed to fetch notifications",
		}, nil
	}

	var notifications []api.NotificationResponse
	for _, notification := range result {
		notifications = append(notifications, convertNotificationToAPI(notification))
	}

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

	stats, err := s.db.Queries().GetNotificationStats(ctx, user.ID)
	if err != nil {
		logger.Error("Failed to get notification stats", "error", err, "user_id", user.ID)
		return api.GetNotificationStats500JSONResponse{
			Code:    500,
			Message: "Failed to get notification stats",
		}, nil
	}

	return api.GetNotificationStats200JSONResponse{
		TotalNotifications:      int(stats.TotalNotifications),
		UnreadCount:             int(stats.UnreadCount),
		HighPriorityCount:       int(stats.HighPriorityCount),
		UnreadHighPriorityCount: int(stats.UnreadHighPriorityCount),
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

	var markedCount int64

	if request.Body.NotificationIds != nil && len(*request.Body.NotificationIds) > 0 {
		for _, notificationID := range *request.Body.NotificationIds {
			_, err := s.db.Queries().MarkNotificationAsRead(ctx, db.MarkNotificationAsReadParams{
				ID:     notificationID,
				UserID: user.ID,
			})
			if err != nil {
				logger.Error("Failed to mark notification as read", "error", err, "notification_id", notificationID, "user_id", user.ID)
				continue
			}
			markedCount++
		}
	} else {
		// count unread notifications since MarkAllNotificationsAsRead doesn't return a count
		unreadCount, err := s.db.Queries().CountUnreadNotifications(ctx, user.ID)
		if err != nil {
			logger.Error("Failed to count unread notifications", "error", err, "user_id", user.ID)
			return api.MarkNotificationsAsRead500JSONResponse{
				Code:    500,
				Message: "Failed to mark notifications as read",
			}, nil
		}

		err = s.db.Queries().MarkAllNotificationsAsRead(ctx, user.ID)
		if err != nil {
			logger.Error("Failed to mark all notifications as read", "error", err, "user_id", user.ID)
			return api.MarkNotificationsAsRead500JSONResponse{
				Code:    500,
				Message: "Failed to mark notifications as read",
			}, nil
		}
		markedCount = unreadCount
	}

	markedCountInt := int(markedCount)

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

	notification, err := s.db.Queries().GetNotification(ctx, request.Id)
	if err != nil {
		if err == sql.ErrNoRows {
			return api.GetNotification404JSONResponse{
				Code:    404,
				Message: "Notification not found",
			}, nil
		}
		logger.Error("Failed to get notification", "error", err, "notification_id", request.Id, "user_id", user.ID)
		return api.GetNotification500JSONResponse{
			Code:    500,
			Message: "Failed to get notification",
		}, nil
	}

	if notification.UserID != user.ID {
		return api.GetNotification403JSONResponse{
			Code:    403,
			Message: "Access denied: notification belongs to another user",
		}, nil
	}

	apiNotification := convertNotificationToAPI(notification)
	return api.GetNotification200JSONResponse(apiNotification), nil
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

	notification, err := s.db.Queries().GetNotification(ctx, request.Id)
	if err != nil {
		if err == sql.ErrNoRows {
			return api.DeleteNotification404JSONResponse{
				Code:    404,
				Message: "Notification not found",
			}, nil
		}
		logger.Error("Failed to check notification existence", "error", err, "notification_id", request.Id, "user_id", user.ID)
		return api.DeleteNotification500JSONResponse{
			Code:    500,
			Message: "Failed to check notification",
		}, nil
	}

	if notification.UserID != user.ID {
		return api.DeleteNotification403JSONResponse{
			Code:    403,
			Message: "Access denied: notification belongs to another user",
		}, nil
	}

	err = s.db.Queries().DeleteNotification(ctx, db.DeleteNotificationParams{
		ID:     request.Id,
		UserID: user.ID,
	})
	if err != nil {
		logger.Error("Failed to delete notification", "error", err, "notification_id", request.Id, "user_id", user.ID)
		return api.DeleteNotification500JSONResponse{
			Code:    500,
			Message: "Failed to delete notification",
		}, nil
	}

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

	body := request.Body

	if body.Title == "" {
		return api.CreateNotification400JSONResponse{
			Code:    400,
			Message: "Title is required",
		}, nil
	}

	if body.Message == "" {
		return api.CreateNotification400JSONResponse{
			Code:    400,
			Message: "Message is required",
		}, nil
	}

	priority := db.NotificationPriorityNormal
	if body.Priority != nil {
		priority = db.NotificationPriority(*body.Priority)
	}

	var metadataBytes []byte
	if body.Metadata != nil {
		metadataBytes, err = json.Marshal(*body.Metadata)
		if err != nil {
			logger.Error("Failed to marshal metadata", "error", err)
			return api.CreateNotification400JSONResponse{
				Code:    400,
				Message: "Invalid metadata format",
			}, nil
		}
	}

	notification, err := s.db.Queries().CreateNotification(ctx, db.CreateNotificationParams{
		UserID:           uuid.UUID(body.UserId),
		Type:             db.NotificationType(body.Type),
		Title:            body.Title,
		Message:          body.Message,
		Priority:         priority,
		ExpiresAt:        timeToPgTimestamp(body.ExpiresAt),
		RelatedBookingID: body.RelatedBookingId,
		RelatedRequestID: body.RelatedRequestId,
		RelatedItemID:    body.RelatedItemId,
		RelatedUserID:    body.RelatedUserId,
		Metadata:         metadataBytes,
	})

	if err != nil {
		logger.Error("Failed to create notification", "error", err, "admin_user_id", user.ID)
		return api.CreateNotification500JSONResponse{
			Code:    500,
			Message: "Failed to create notification",
		}, nil
	}

	logger.Info("Notification created successfully",
		"notification_id", notification.ID,
		"user_id", notification.UserID,
		"type", notification.Type,
		"priority", notification.Priority,
	)

	apiNotification := convertNotificationToAPI(notification)
	return api.CreateNotification201JSONResponse(apiNotification), nil
}
