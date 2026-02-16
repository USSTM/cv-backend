package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/USSTM/cv-backend/internal/aws"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/logging"
	"github.com/USSTM/cv-backend/internal/queue"
)

func main() {
	cfg := config.Load()

	if err := logging.Init(&cfg.Logging); err != nil {
		logging.Error("Failed to initialize logger: %v", err)
	}

	emailSvc, err := aws.NewEmailService(cfg.AWS)
	if err != nil {
		logging.Error("Failed to initialize email service: %v", err)
	}

	logging.Info("Verifying sender identity", "email", emailSvc.Sender())
	if _, err := emailSvc.VerifyEmailIdentity(context.Background()); err != nil {
		logging.Error("Failed to verify email identity: %v", err)
	}

	worker := queue.NewWorker(&cfg.Redis, emailSvc)

	logging.Info("Starting queue worker...")
	if err := worker.Start(); err != nil {
		logging.Error("Worker failed to start: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logging.Info("Shutting down worker...")
	worker.Close()
}
