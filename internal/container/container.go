package container

import (
	"context"

	"github.com/USSTM/cv-backend/internal/api"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/aws"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/database"
	"github.com/USSTM/cv-backend/internal/logging"
	"github.com/USSTM/cv-backend/internal/queue"
)

type Container struct {
	Config        *config.Config
	Database      *database.Database
	Queue         *queue.TaskQueue
	JWTService    *auth.JWTService
	EmailService  *aws.EmailService
	S3Service     *aws.S3Service
	Authenticator *auth.Authenticator
	Server        *api.Server
	Worker        *queue.Worker
}

func New(cfg config.Config) (*Container, error) {
	db, err := database.New(&cfg.Database)
	if err != nil {
		return nil, err
	}

	taskQueue, err := queue.NewQueue(&cfg.Redis)
	if err != nil {
		return nil, err
	}

	jwtService, err := auth.NewJWTService([]byte(cfg.JWT.SigningKey), cfg.JWT.Issuer, cfg.JWT.Expiry)
	if err != nil {
		return nil, err
	}

	authenticator := auth.NewAuthenticator(jwtService, db.Queries())

	sesService, err := aws.NewEmailService(cfg.AWS)
	if err != nil {
		return nil, err
	}

	// localstack-specific config (email identity not managed by app in prod)
	if cfg.AWS.EndpointURL != "" {
		if _, err := sesService.VerifyEmailIdentity(context.Background()); err != nil {
			logging.Error("Failed to verify email identity", "error", err)
		}
	}

	s3Service, err := aws.NewS3Service(cfg.AWS)
	if err != nil {
		return nil, err
	}

	// localstack-specific config (buckets are not managed by app in prod)
	if cfg.AWS.EndpointURL != "" {
		if err := s3Service.CreateBucket(context.Background()); err != nil {
			logging.Info("S3 bucket creation attempted", "bucket", cfg.AWS.Bucket, "result", err)
		}
	}

	worker := queue.NewWorker(&cfg.Redis, sesService)

	server := api.NewServer(db, taskQueue, jwtService, authenticator, sesService, s3Service)

	logging.Info("Connected to database",
		"host", cfg.Database.Host,
		"port", cfg.Database.Port)

	return &Container{
		Config:        &cfg,
		Database:      db,
		Queue:         taskQueue,
		JWTService:    jwtService,
		EmailService:  sesService,
		S3Service:     s3Service,
		Authenticator: authenticator,
		Server:        server,
		Worker:        worker,
	}, nil
}

func (c *Container) Cleanup() {
	if c.Database != nil {
		c.Database.Close()
		logging.Info("Database connection closed")
	}
	if c.Worker != nil {
		c.Worker.Close()
		logging.Info("Worker closed")
	}
}
