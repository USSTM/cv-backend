package api

import (
	"context"
	"errors"
	"testing"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_LoginUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create test database
	testDB := testutil.NewTestDatabase(t)
	testDB.RunMigrations(t) // This includes seeding from the migration

	// Create mock services
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)

	// Create server with real database and mocked services
	server := NewServer(testDB, mockJWT, mockAuth)

	t.Run("successful login", func(t *testing.T) {
		// Create test user in database using builder
		testUser := testDB.NewUser(t).
			WithEmail("test@example.com").
			AsMember().
			Create()

		// Set up mock expectations
		mockJWT.ExpectGenerateToken(testUser.ID, "test-token", nil)

		// Create request using StrictServerInterface pattern
		request := api.LoginUserRequestObject{
			Body: &api.LoginUserJSONRequestBody{
				Email: types.Email(testUser.Email),
			},
		}

		// Call handler directly
		response, err := server.LoginUser(context.Background(), request)

		// Assert response
		require.NoError(t, err)
		require.IsType(t, api.LoginUser200JSONResponse{}, response)

		loginResp := response.(api.LoginUser200JSONResponse)
		assert.NotNil(t, loginResp.Token)
		assert.Equal(t, "test-token", *loginResp.Token)

		// Verify mock was called
		mockJWT.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		// Create request with non-existent user
		request := api.LoginUserRequestObject{
			Body: &api.LoginUserJSONRequestBody{
				Email: types.Email("nonexistent@example.com"),
			},
		}

		// Call handler directly
		response, err := server.LoginUser(context.Background(), request)

		// Assert error response
		require.NoError(t, err)
		require.IsType(t, api.LoginUser400JSONResponse{}, response)

		errorResp := response.(api.LoginUser400JSONResponse)
		assert.Equal(t, int32(400), errorResp.Code)
		assert.Equal(t, "Invalid email or password.", errorResp.Message)
	})

	t.Run("jwt generation failure", func(t *testing.T) {
		// Create test user with unique email
		testUser := testDB.NewUser(t).
			WithEmail("failure@example.com").
			AsMember().
			Create()

		// Set up mock to return error
		mockJWT.ExpectGenerateToken(testUser.ID, "", errors.New("jwt generation failed"))

		// Create request
		request := api.LoginUserRequestObject{
			Body: &api.LoginUserJSONRequestBody{
				Email: types.Email(testUser.Email),
			},
		}

		// Call handler directly
		response, err := server.LoginUser(context.Background(), request)

		// Assert error response
		require.NoError(t, err)
		require.IsType(t, api.LoginUser500JSONResponse{}, response)

		errorResp := response.(api.LoginUser500JSONResponse)
		assert.Equal(t, int32(500), errorResp.Code)
		assert.Equal(t, "An unexpected error occurred.", errorResp.Message)

		// Verify mock was called
		mockJWT.AssertExpectations(t)
	})
}

func TestServer_PingProtected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	testDB := testutil.NewTestDatabase(t)
	testDB.RunMigrations(t)

	mockJWT := &testutil.MockJWTService{}
	mockAuth := &testutil.MockAuthenticator{}

	server := NewServer(testDB, mockJWT, mockAuth)

	t.Run("successful ping with authenticated user", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("test@example.com").
			AsMember().
			Create()

		// Mock permission check
		mockAuth.ExpectCheckPermission(testUser.ID, "view_own_data", nil, true, nil)

		// Create context with authenticated user
		ctx := context.WithValue(context.Background(), auth.UserClaimsKey, &auth.AuthenticatedUser{
			ID:    testUser.ID,
			Email: testUser.Email,
		})

		// Call handler directly
		response, err := server.PingProtected(ctx, api.PingProtectedRequestObject{})

		// Assert response
		require.NoError(t, err)
		require.IsType(t, api.PingProtected200JSONResponse{}, response)

		pingResp := response.(api.PingProtected200JSONResponse)
		assert.Contains(t, pingResp.Message, "PONG! Hello")
		assert.Contains(t, pingResp.Message, testUser.Email)
		assert.NotZero(t, pingResp.Timestamp)
	})

	t.Run("unauthorized user", func(t *testing.T) {
		// Context without authenticated user
		ctx := context.Background()

		response, err := server.PingProtected(ctx, api.PingProtectedRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.PingProtected401JSONResponse{}, response)

		errorResp := response.(api.PingProtected401JSONResponse)
		assert.Equal(t, int32(401), errorResp.Code)
		assert.Equal(t, "Unauthorized!", errorResp.Message)
	})

	t.Run("insufficient permissions", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("test2@example.com").
			AsMember().
			Create()

		// Mock permission check to return false
		mockAuth.ExpectCheckPermission(testUser.ID, "view_own_data", nil, false, nil)

		ctx := context.WithValue(context.Background(), auth.UserClaimsKey, &auth.AuthenticatedUser{
			ID:    testUser.ID,
			Email: testUser.Email,
		})

		response, err := server.PingProtected(ctx, api.PingProtectedRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.PingProtected401JSONResponse{}, response)

		errorResp := response.(api.PingProtected401JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Equal(t, "Insufficient permissions", errorResp.Message)
	})
}
