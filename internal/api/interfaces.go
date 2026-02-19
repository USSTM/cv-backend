package api

import (
	"context"
	"io"
	"time"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseService defines the interface for database operations
type DatabaseService interface {
	Queries() *db.Queries
	Pool() *pgxpool.Pool
	Close()
}

// AuthService defines the interface for passwordless OTP + refresh token auth
type AuthService interface {
	RequestOTP(ctx context.Context, email string) (string, error)
	VerifyOTP(ctx context.Context, email, code string) (string, string, error)
	Refresh(ctx context.Context, refreshToken string) (string, string, error)
	Logout(ctx context.Context, refreshToken string) error
	OTPExpiry() time.Duration
}

// AuthenticatorService defines the interface for authentication operations
type AuthenticatorService interface {
	CheckPermission(ctx context.Context, userID uuid.UUID, permission string, scopeID *uuid.UUID) (bool, error)
}

// RedisQueueService defines the interface for Redis (asynq) queue operations
type RedisQueueService interface {
	Enqueue(taskType string, data interface{}) (*asynq.TaskInfo, error)
}

// EmailService defines the interface for email operations
type EmailService interface {
	SendEmail(ctx context.Context, to string, subject string, body string) error
}

// S3Service defines the interface for S3 operations
type S3Service interface {
	PutObject(ctx context.Context, key string, body io.Reader, contentType string) error
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	GeneratePresignedURL(ctx context.Context, method string, key string, duration time.Duration) (string, error)
}
