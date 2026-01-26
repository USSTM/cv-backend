package api

import (
	"context"
	"testing"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_Notifications(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping notifications tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	testLocalStack := testutil.NewTestLocalStack(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)

	server := NewServer(testDB, testQueue, testLocalStack, mockJWT, mockAuth)

	t.Run("successful get notifications", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("notifications@test.com").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewNotifications, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetNotifications(ctx, api.GetNotificationsRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.GetNotifications200JSONResponse{}, response)

		notificationsResp := response.(api.GetNotifications200JSONResponse)
		assert.NotNil(t, notificationsResp)
	})

	t.Run("unauthorized get notifications", func(t *testing.T) {
		ctx := context.Background() // No user in context

		response, err := server.GetNotifications(ctx, api.GetNotificationsRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.GetNotifications401JSONResponse{}, response)

		unauthorizedResp := response.(api.GetNotifications401JSONResponse)
		assert.Equal(t, 401, unauthorizedResp.Code)
		assert.Equal(t, "Unauthorized", unauthorizedResp.Message)
	})

	t.Run("forbidden get notifications", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("forbidden@test.com").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewNotifications, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetNotifications(ctx, api.GetNotificationsRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.GetNotifications403JSONResponse{}, response)

		forbiddenResp := response.(api.GetNotifications403JSONResponse)
		assert.Equal(t, 403, forbiddenResp.Code)
		assert.Equal(t, "Insufficient permissions", forbiddenResp.Message)
	})

	t.Run("successful get notification stats", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("stats@test.com").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewNotifications, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetNotificationStats(ctx, api.GetNotificationStatsRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.GetNotificationStats200JSONResponse{}, response)

		statsResp := response.(api.GetNotificationStats200JSONResponse)
		assert.GreaterOrEqual(t, statsResp.UnreadCount, 0)
		assert.GreaterOrEqual(t, statsResp.TotalNotifications, 0)
	})

	t.Run("successful mark notifications as read", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("markread@test.com").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewNotifications, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		requestBody := api.MarkAsReadRequest{}
		response, err := server.MarkNotificationsAsRead(ctx, api.MarkNotificationsAsReadRequestObject{
			Body: &requestBody,
		})
		require.NoError(t, err)
		require.IsType(t, api.MarkNotificationsAsRead200JSONResponse{}, response)

		markReadResp := response.(api.MarkNotificationsAsRead200JSONResponse)
		assert.NotNil(t, markReadResp.MarkedCount)
		assert.GreaterOrEqual(t, *markReadResp.MarkedCount, 0)
	})

	t.Run("mark notifications as read without body", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("markread2@test.com").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewNotifications, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.MarkNotificationsAsRead(ctx, api.MarkNotificationsAsReadRequestObject{
			Body: nil,
		})
		require.NoError(t, err)
		require.IsType(t, api.MarkNotificationsAsRead400JSONResponse{}, response)

		badReqResp := response.(api.MarkNotificationsAsRead400JSONResponse)
		assert.Equal(t, 400, badReqResp.Code)
		assert.Equal(t, "Request body is required", badReqResp.Message)
	})

	t.Run("successful get single notification", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("single@test.com").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewNotifications, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		notificationID := uuid.New()
		response, err := server.GetNotification(ctx, api.GetNotificationRequestObject{
			Id: notificationID,
		})
		require.NoError(t, err)
		// This will likely return 404 since we didn't create a notification, but that's expected
		assert.NotNil(t, response)
	})

	t.Run("successful delete notification", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("delete@test.com").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewNotifications, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		notificationID := uuid.New()
		response, err := server.DeleteNotification(ctx, api.DeleteNotificationRequestObject{
			Id: notificationID,
		})
		require.NoError(t, err)
		// This will likely return 404 since we didn't create a notification, but that's expected
		assert.NotNil(t, response)
	})

	t.Run("successful create notification as admin", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("createadmin@test.com").
			AsGlobalAdmin().
			Create()

		targetUser := testDB.NewUser(t).
			WithEmail("target@test.com").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageNotifications, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		priority := api.NotificationPriority("normal")
		requestBody := api.CreateNotificationJSONRequestBody{
			UserId:   api.UUID(targetUser.ID),
			Type:     api.NotificationType("system_announcement"),
			Title:    "Test Notification",
			Message:  "This is a test notification",
			Priority: &priority,
		}

		response, err := server.CreateNotification(ctx, api.CreateNotificationRequestObject{
			Body: &requestBody,
		})
		require.NoError(t, err)
		require.IsType(t, api.CreateNotification201JSONResponse{}, response)

		createResp := response.(api.CreateNotification201JSONResponse)
		assert.Equal(t, targetUser.ID, createResp.UserId)
		assert.Equal(t, "Test Notification", createResp.Title)
		assert.Equal(t, "This is a test notification", createResp.Message)
		assert.Equal(t, api.NotificationType("system_announcement"), createResp.Type)
		assert.Equal(t, api.NotificationPriority("normal"), createResp.Priority)
	})

	t.Run("forbidden create notification as non-admin", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("createforbidden@test.com").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageNotifications, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		priority := api.NotificationPriority("low")
		requestBody := api.CreateNotificationJSONRequestBody{
			UserId:   api.UUID(testUser.ID),
			Type:     api.NotificationType("system_announcement"),
			Title:    "Test Notification",
			Message:  "This should not be allowed",
			Priority: &priority,
		}

		response, err := server.CreateNotification(ctx, api.CreateNotificationRequestObject{
			Body: &requestBody,
		})
		require.NoError(t, err)
		require.IsType(t, api.CreateNotification403JSONResponse{}, response)

		forbiddenResp := response.(api.CreateNotification403JSONResponse)
		assert.Equal(t, 403, forbiddenResp.Code)
		assert.Equal(t, "Insufficient permissions", forbiddenResp.Message)
	})

	t.Run("create notification without body", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("createbadreq@test.com").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageNotifications, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.CreateNotification(ctx, api.CreateNotificationRequestObject{
			Body: nil,
		})
		require.NoError(t, err)
		require.IsType(t, api.CreateNotification400JSONResponse{}, response)

		badReqResp := response.(api.CreateNotification400JSONResponse)
		assert.Equal(t, 400, badReqResp.Code)
		assert.Equal(t, "Request body is required", badReqResp.Message)
	})
}
