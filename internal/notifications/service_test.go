package notifications_test

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/USSTM/cv-backend/internal/notifications"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	sharedDB    *testutil.TestDatabase
	sharedQueue *testutil.TestQueue
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(0)
	}

	t := &testing.T{}
	sharedDB = testutil.NewTestDatabase(t)
	sharedDB.RunMigrations(t)
	sharedQueue = testutil.NewTestQueue(t)

	code := m.Run()

	if sharedDB.Pool() != nil {
		sharedDB.Pool().Close()
	}
	sharedQueue.Close()

	os.Exit(code)
}

func newTestNotificationService(t *testing.T) *notifications.NotificationService {
	t.Helper()
	return notifications.NewNotificationService(sharedDB.Pool(), sharedDB.Queries())
}

func TestNotificationService_PublishAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx := context.Background()

	t.Run("publishes and retrieves notification", func(t *testing.T) {
		sharedDB.CleanupDatabase(t)
		svc := newTestNotificationService(t)

		actor := sharedDB.NewUser(t).WithEmail("actor@example.com").Create()
		notifier := sharedDB.NewUser(t).WithEmail("notifier@example.com").Create()

		entityID := uuid.New()

		err := svc.Publish(ctx, actor.ID, "general", entityID, []uuid.UUID{notifier.ID})
		require.NoError(t, err)

		notifs, err := svc.GetUserNotifications(ctx, notifier.ID, 10, 0)
		require.NoError(t, err)

		assert.Len(t, notifs, 1)
		n := notifs[0]
		assert.False(t, n.IsRead)
		assert.Equal(t, actor.ID, n.ActorID)
		assert.Equal(t, entityID, n.EntityID)
		assert.Equal(t, "general", n.EntityTypeName)

		count, err := svc.GetUnreadCount(ctx, notifier.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)

		marked, err := svc.MarkAsRead(ctx, notifier.ID, n.NotificationID)
		require.NoError(t, err)
		assert.True(t, marked.IsRead)

		countAfterRead, err := svc.GetUnreadCount(ctx, notifier.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), countAfterRead)
	})

	t.Run("mark all as read", func(t *testing.T) {
		sharedDB.CleanupDatabase(t)
		svc := newTestNotificationService(t)

		actor := sharedDB.NewUser(t).WithEmail("actor2@example.com").Create()
		notifier := sharedDB.NewUser(t).WithEmail("notifier2@example.com").Create()

		// publish two notifications
		err := svc.Publish(ctx, actor.ID, "general", uuid.New(), []uuid.UUID{notifier.ID})
		require.NoError(t, err)

		err = svc.Publish(ctx, actor.ID, "general", uuid.New(), []uuid.UUID{notifier.ID})
		require.NoError(t, err)

		count, err := svc.GetUnreadCount(ctx, notifier.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// mark all as read
		err = svc.MarkAllAsRead(ctx, notifier.ID)
		require.NoError(t, err)

		countAfter, err := svc.GetUnreadCount(ctx, notifier.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), countAfter)
	})
}
