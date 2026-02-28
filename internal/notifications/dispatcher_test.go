package notifications_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/USSTM/cv-backend/internal/notifications"
	"github.com/USSTM/cv-backend/internal/queue"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDispatcher(t *testing.T) *notifications.NotificationDispatcher {
	t.Helper()
	svc := notifications.NewNotificationService(sharedDB.Pool(), sharedDB.Queries())
	emailTemplates, err := notifications.LoadTemplates("../../templates/email")
	require.NoError(t, err)
	return notifications.NewNotificationDispatcher(svc, sharedQueue, emailTemplates, notifications.NewEmailLookupFunc(sharedDB.Queries()))
}

func TestNotificationDispatcher_Notify_InAppOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	sharedDB.CleanupDatabase(t)
	sharedQueue.Cleanup(t)

	ctx := context.Background()
	actor := sharedDB.NewUser(t).WithEmail("actor@example.com").Create()
	notifier := sharedDB.NewUser(t).WithEmail("notifier@example.com").Create()

	d := newTestDispatcher(t)
	entityID := uuid.New()

	err := d.Notify(ctx, actor.ID, "general", entityID, []notifications.NotifierGroup{
		{IDs: []uuid.UUID{notifier.ID}, Template: ""},
	})
	require.NoError(t, err)

	notifs, err := d.GetUserNotifications(ctx, notifier.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, notifs, 1)
	assert.Equal(t, entityID, notifs[0].EntityID)

	// queue not found means no tasks were ever enqueued — treat as empty
	tasks, _ := sharedQueue.Inspector.ListPendingTasks("default")
	assert.Empty(t, tasks, "no email tasks should be enqueued for in-app only group")
}

func TestNotificationDispatcher_Notify_WithEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	sharedDB.CleanupDatabase(t)
	sharedQueue.Cleanup(t)

	ctx := context.Background()
	actor := sharedDB.NewUser(t).WithEmail("actor2@example.com").Create()
	notifier := sharedDB.NewUser(t).WithEmail("notifier2@example.com").Create()

	d := newTestDispatcher(t)
	entityID := uuid.New()

	err := d.Notify(ctx, actor.ID, "general", entityID, []notifications.NotifierGroup{
		{
			IDs:      []uuid.UUID{notifier.ID},
			Template: "request_approved_requester",
			TemplateData: map[string]interface{}{
				"UserName":  "Test User",
				"ItemName":  "Test Item",
				"RequestID": entityID.String(),
			},
		},
	})
	require.NoError(t, err)

	notifs, err := d.GetUserNotifications(ctx, notifier.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, notifs, 1)

	tasks, err := sharedQueue.Inspector.ListPendingTasks("default")
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, queue.TypeEmailDelivery, tasks[0].Type)

	var payload queue.EmailDeliveryPayload
	require.NoError(t, json.Unmarshal(tasks[0].Payload, &payload))
	assert.Equal(t, "notifier2@example.com", payload.To)
	assert.Contains(t, payload.Subject, "Test Item")
}

func TestNotificationDispatcher_Notify_MultiGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	sharedDB.CleanupDatabase(t)
	sharedQueue.Cleanup(t)

	ctx := context.Background()
	actor := sharedDB.NewUser(t).WithEmail("actor4@example.com").Create()
	requester := sharedDB.NewUser(t).WithEmail("requester@example.com").Create()
	approver := sharedDB.NewUser(t).WithEmail("approver@example.com").Create()

	d := newTestDispatcher(t)
	entityID := uuid.New()

	err := d.Notify(ctx, actor.ID, "general", entityID, []notifications.NotifierGroup{
		{
			IDs:      []uuid.UUID{requester.ID},
			Template: "request_approved_requester",
			TemplateData: map[string]interface{}{
				"UserName":  "Requester",
				"ItemName":  "Laptop",
				"RequestID": entityID.String(),
			},
		},
		{
			IDs:      []uuid.UUID{approver.ID},
			Template: "request_approved_approver",
			TemplateData: map[string]interface{}{
				"UserName":      "Approver",
				"RequesterName": "Requester",
				"ItemName":      "Laptop",
			},
		},
	})
	require.NoError(t, err)

	// both receive in-app notifications
	requesterNotifs, err := d.GetUserNotifications(ctx, requester.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, requesterNotifs, 1)

	approverNotifs, err := d.GetUserNotifications(ctx, approver.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, approverNotifs, 1)

	// one email per group
	tasks, err := sharedQueue.Inspector.ListPendingTasks("default")
	require.NoError(t, err)
	require.Len(t, tasks, 2)

	recipients := make([]string, 0, 2)
	for _, task := range tasks {
		var payload queue.EmailDeliveryPayload
		require.NoError(t, json.Unmarshal(task.Payload, &payload))
		recipients = append(recipients, payload.To)
	}
	assert.ElementsMatch(t, []string{"requester@example.com", "approver@example.com"}, recipients)
}

func TestNotificationDispatcher_Notify_ActorSkippedInApp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	sharedDB.CleanupDatabase(t)
	sharedQueue.Cleanup(t)

	ctx := context.Background()
	actor := sharedDB.NewUser(t).WithEmail("actor3@example.com").Create()

	d := newTestDispatcher(t)

	err := d.Notify(ctx, actor.ID, "general", uuid.New(), []notifications.NotifierGroup{
		{IDs: []uuid.UUID{actor.ID}, Template: ""},
	})
	require.NoError(t, err)

	notifs, err := d.GetUserNotifications(ctx, actor.ID, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, notifs, "actor should not receive their own in-app notification")
}
