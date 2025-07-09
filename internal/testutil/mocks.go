package testutil

import (
	"context"
	"testing"

	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockJWTService is a mock implementation of the JWT service interface
type MockJWTService struct {
	mock.Mock
}

// NewMockJWTService creates a new mock JWT service
func NewMockJWTService(t *testing.T) *MockJWTService {
	mockJWT := &MockJWTService{}
	mockJWT.Test(t)
	return mockJWT
}

// GenerateToken mocks token generation
func (m *MockJWTService) GenerateToken(ctx context.Context, userID uuid.UUID) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

// ValidateToken mocks token validation
func (m *MockJWTService) ValidateToken(ctx context.Context, token string) (*auth.TokenClaims, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(*auth.TokenClaims), args.Error(1)
}

// MockAuthenticator is a mock implementation of the authenticator interface
type MockAuthenticator struct {
	mock.Mock
}

// NewMockAuthenticator creates a new mock authenticator
func NewMockAuthenticator(t *testing.T) *MockAuthenticator {
	mockAuth := &MockAuthenticator{}
	mockAuth.Test(t)
	return mockAuth
}

// CheckPermission mocks permission checking
func (m *MockAuthenticator) CheckPermission(ctx context.Context, userID uuid.UUID, permission string, scopeID *uuid.UUID) (bool, error) {
	args := m.Called(ctx, userID, permission, scopeID)
	return args.Bool(0), args.Error(1)
}

// Helper methods for setting up common mock expectations

// ExpectGenerateToken sets up expectation for GenerateToken
func (m *MockJWTService) ExpectGenerateToken(userID uuid.UUID, token string, err error) *mock.Call {
	return m.On("GenerateToken", mock.Anything, userID).Return(token, err)
}

// ExpectValidateToken sets up expectation for ValidateToken
func (m *MockJWTService) ExpectValidateToken(token string, claims *auth.TokenClaims, err error) *mock.Call {
	return m.On("ValidateToken", mock.Anything, token).Return(claims, err)
}

// ExpectCheckPermission sets up expectation for CheckPermission
func (m *MockAuthenticator) ExpectCheckPermission(userID uuid.UUID, permission string, scopeID *uuid.UUID, hasPermission bool, err error) *mock.Call {
	return m.On("CheckPermission", mock.Anything, userID, permission, scopeID).Return(hasPermission, err)
}
