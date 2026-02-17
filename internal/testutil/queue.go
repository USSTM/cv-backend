package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/queue"
	"github.com/hibiken/asynq"
	rdb "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestQueue struct {
	Queue     *queue.TaskQueue
	container *redis.RedisContainer
	Redis     *rdb.Client
	Inspector *asynq.Inspector // (this is for inspecting the queue in tests)
}

func NewTestQueue(t *testing.T) *TestQueue {
	ctx := context.Background()

	// Create Redis container with reuse enabled
	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithReuseByName("cv-backend-test-redis"),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("Ready to accept connections").
					WithStartupTimeout(30*time.Second),
				wait.ForListeningPort("6379/tcp").
					WithStartupTimeout(30*time.Second),
			),
		),
	)
	require.NoError(t, err, "Failed to start Redis container")

	// Get connection string
	endpoint, err := redisContainer.Endpoint(ctx, "")
	require.NoError(t, err, "Failed to get redis connection string")

	// Create redis Options manually
	clientOpts := asynq.RedisClientOpt{
		Addr:     endpoint,
		Password: "",
		DB:       0,
	}

	appConfig := config.RedisConfig{
		Addr:     endpoint,
		Password: "",
		DB:       0,
	}

	// Create Asynq Task Queue
	taskQueue, err := queue.NewQueue(&appConfig)
	require.NoError(t, err, "Failed to create application queue wrapper")

	// Create Asynq Inspector
	inspector := asynq.NewInspector(clientOpts)

	// create a redis client for direct access
	redisClient := rdb.NewClient(&rdb.Options{
		Addr: endpoint,
	})

	testQueue := &TestQueue{
		Queue:     taskQueue,
		container: redisContainer,
		Redis:     redisClient,
		Inspector: inspector,
	}

	return testQueue
}

func (tQ *TestQueue) Enqueue(taskType string, data interface{}) (*asynq.TaskInfo, error) {
	return tQ.Queue.Enqueue(taskType, data)
}

func (tQ *TestQueue) Cleanup(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tQ.Redis.FlushDB(ctx).Err(); err != nil {
		t.Logf("WARNING: failed to flush Redis between tests: %v", err)
	}
}

func (tq *TestQueue) Close() {
	if tq.Queue != nil {
		tq.Queue.Close()
	}
	if tq.Inspector != nil {
		tq.Inspector.Close()
	}
	if tq.Redis != nil {
		tq.Redis.Close()
	}
}
