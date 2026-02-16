package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/USSTM/cv-backend/internal/aws"
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

const (
	TypeEmailDelivery = "email:delivery"
)

type EmailDeliveryPayload struct {
	To      string
	Subject string
	Body    string
}

type Worker struct {
	server       *asynq.Server
	emailService aws.EmailService
}

func NewWorker(cfg *config.RedisConfig, emailService *aws.EmailService) *Worker {
	server := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		},
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logging.Error("process task failed", "type", task.Type(), "payload", string(task.Payload()), "error", err)
			}),
		},
	)

	return &Worker{
		server:       server,
		emailService: *emailService,
	}
}

func (w *Worker) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeEmailDelivery, w.HandleEmailDelivery)

	return w.server.Start(mux)
}

func (w *Worker) Close() {
	if w.server != nil {
		w.server.Shutdown()
	}
}

func (w *Worker) HandleEmailDelivery(ctx context.Context, t *asynq.Task) error {
	var p EmailDeliveryPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	logging.Info("Sending email", "to", p.To, "subject", p.Subject)
	if err := w.emailService.SendEmail(ctx, p.To, p.Subject, p.Body); err != nil {
		return fmt.Errorf("emailService.SendEmail failed: %w", err)
	}

	return nil
}
