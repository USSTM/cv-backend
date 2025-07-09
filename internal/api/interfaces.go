package api

import (
	"context"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/google/uuid"
)

// DatabaseService defines the interface for database operations
type DatabaseService interface {
	Queries() *db.Queries
	Close()
}

// JWTService defines the interface for JWT operations
type JWTService interface {
	GenerateToken(ctx context.Context, userID uuid.UUID) (string, error)
	ValidateToken(ctx context.Context, token string) (*auth.TokenClaims, error)
}

// AuthenticatorService defines the interface for authentication operations
type AuthenticatorService interface {
	CheckPermission(ctx context.Context, userID uuid.UUID, permission string, scopeID *uuid.UUID) (bool, error)
}
