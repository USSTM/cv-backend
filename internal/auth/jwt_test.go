package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTService_GenerateToken(t *testing.T) {
	service, err := NewJWTService([]byte("test-secret-key"), "test-issuer", time.Hour)
	require.NoError(t, err)

	userID := uuid.New()
	ctx := context.Background()

	token, err := service.GenerateToken(ctx, userID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Contains(t, token, ".")
}

func TestJWTService_ValidateToken(t *testing.T) {
	service, err := NewJWTService([]byte("test-secret-key"), "test-issuer", time.Hour)
	require.NoError(t, err)

	userID := uuid.New()
	ctx := context.Background()

	token, err := service.GenerateToken(ctx, userID)
	require.NoError(t, err)

	claims, err := service.ValidateToken(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
}

func TestJWTService_ValidateToken_InvalidToken(t *testing.T) {
	service, err := NewJWTService([]byte("test-secret-key"), "test-issuer", time.Hour)
	require.NoError(t, err)

	ctx := context.Background()

	_, err = service.ValidateToken(ctx, "invalid-token")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse token")
}

func TestJWTService_ValidateToken_WrongSecret(t *testing.T) {
	service1, err := NewJWTService([]byte("secret-1"), "test-issuer", time.Hour)
	require.NoError(t, err)

	service2, err := NewJWTService([]byte("secret-2"), "test-issuer", time.Hour)
	require.NoError(t, err)

	userID := uuid.New()
	ctx := context.Background()

	token, err := service1.GenerateToken(ctx, userID)
	require.NoError(t, err)

	_, err = service2.ValidateToken(ctx, token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse token")
}

func TestJWTService_ValidateToken_ExpiredToken(t *testing.T) {
	service, err := NewJWTService([]byte("test-secret-key"), "test-issuer", time.Millisecond)
	require.NoError(t, err)

	userID := uuid.New()
	ctx := context.Background()

	token, err := service.GenerateToken(ctx, userID)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	_, err = service.ValidateToken(ctx, token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse token")
}
