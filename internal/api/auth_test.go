package api

import (
	"context"
	"errors"
	"testing"

	"github.com/USSTM/cv-backend/internal/rbac"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAuthTestServer is a local newTestServer impl that returns the JWT mock,
// needed for tests that assert on token generation.
func newAuthTestServer(t *testing.T) (*Server, *testutil.TestDatabase, *testutil.MockAuthenticator, *testutil.MockJWTService) {
	testDB := getSharedTestDatabase(t)
	sharedQueue.Cleanup(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, sharedQueue, mockJWT, mockAuth, sharedLocalStack, sharedLocalStack)
	return server, testDB, mockAuth, mockJWT
}

func TestServer_LoginUser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, _, mockJWT := newAuthTestServer(t)

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
				Email:    types.Email(testUser.Email),
				Password: "password",
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
				Email:    types.Email("nonexistent@example.com"),
				Password: "password",
			},
		}

		// Call handler directly
		response, err := server.LoginUser(context.Background(), request)

		// Assert error response
		require.NoError(t, err)
		require.IsType(t, api.LoginUser400JSONResponse{}, response)

		errorResp := response.(api.LoginUser400JSONResponse)
		assert.Equal(t, "VALIDATION_ERROR", string(errorResp.Error.Code))
		assert.Equal(t, "Invalid email or password.", errorResp.Error.Message)
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
				Email:    types.Email(testUser.Email),
				Password: "password",
			},
		}

		// Call handler directly
		response, err := server.LoginUser(context.Background(), request)

		// Assert error response
		require.NoError(t, err)
		require.IsType(t, api.LoginUser500JSONResponse{}, response)

		errorResp := response.(api.LoginUser500JSONResponse)
		assert.Equal(t, "INTERNAL_ERROR", string(errorResp.Error.Code))
		assert.Equal(t, "An unexpected error occurred.", errorResp.Error.Message)

		// Verify mock was called
		mockJWT.AssertExpectations(t)
	})
}

func TestServer_PingProtected(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, mockAuth := newTestServer(t)

	t.Run("successful ping with authenticated user", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("test@example.com").
			AsMember().
			Create()

		// Mock permission check
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

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
		assert.Equal(t, "AUTHENTICATION_REQUIRED", string(errorResp.Error.Code))
		assert.Equal(t, "Authentication required", errorResp.Error.Message)
	})

	t.Run("insufficient permissions", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("test2@example.com").
			AsMember().
			Create()

		// Mock permission check to return false
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, false, nil)

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.PingProtected(ctx, api.PingProtectedRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.PingProtected401JSONResponse{}, response)

		errorResp := response.(api.PingProtected401JSONResponse)
		assert.Equal(t, "PERMISSION_DENIED", string(errorResp.Error.Code))
		assert.Equal(t, "Insufficient permissions", errorResp.Error.Message)
	})
}
