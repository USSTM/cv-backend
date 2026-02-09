package main

import (
	"context"
	"log"

	"github.com/USSTM/cv-backend/internal/aws"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/logging"
	"github.com/USSTM/cv-backend/internal/queue"
)

func main() {
	cfg := config.Load()

	if err := logging.Init(&cfg.Logging); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	emailSvc, err := aws.NewEmailService(cfg.AWS)
	if err != nil {
		log.Fatalf("Failed to initialize email service: %v", err)
	}

	log.Printf("Verifying sender identity %s...", emailSvc.Sender())
	if _, err := emailSvc.VerifyEmailIdentity(context.Background()); err != nil {
		log.Fatalf("Failed to verify email identity: %v", err)
	}

	worker := queue.NewWorker(&cfg.Redis, emailSvc)

	log.Println("Starting queue worker...")
	if err := worker.Start(); err != nil {
		log.Fatalf("Worker failed to start: %v", err)
	}
}
