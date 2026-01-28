package queue

import (
	"encoding/json"
	"fmt"

	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/logging"
	"github.com/hibiken/asynq"
)

type TaskQueue struct {
	client *asynq.Client
}

func NewQueue(cfg *config.RedisConfig) (*TaskQueue, error) {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Activate and test the connection
	if err := client.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis queue: %w", err)
	}

	logging.Info("Connected to Redis task queue")

	return &TaskQueue{client: client}, nil
}

func (q *TaskQueue) Enqueue(taskType string, data interface{}) (*asynq.TaskInfo, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	task := asynq.NewTask(taskType, payload)

	t, err := q.client.Enqueue(task)

	return t, err
}

func (q *TaskQueue) Close() error {
	return q.client.Close()
}
