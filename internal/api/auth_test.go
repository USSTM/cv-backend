package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
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
		// Create test user in database
		testUser := testutil.NewTestUser()
		testDB.CreateTestUser(t, testUser)

		// Set up mock expectations
		mockJWT.ExpectGenerateToken(testUser.ID, "test-token", nil)

		// Create request body
		loginReq := api.LoginRequest{
			Email: types.Email(testUser.Email),
		}

		// Create HTTP request
		req := testutil.Request{
			Method: "POST",
			Path:   "/login",
			Body:   loginReq,
		}

		// Create test server and make request
		ts := testutil.NewTestServer(t, http.HandlerFunc(server.LoginUser))
		resp := ts.MakeRequest(t, req)

		// Assert response
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.NotNil(t, resp.Body["token"])
		assert.Equal(t, "test-token", resp.Body["token"])

		// Verify mock was called
		mockJWT.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		// Make request with non-existent user
		loginReq := api.LoginRequest{
			Email: types.Email("nonexistent@example.com"),
		}

		req := testutil.Request{
			Method: "POST",
			Path:   "/login",
			Body:   loginReq,
		}

		ts := testutil.NewTestServer(t, http.HandlerFunc(server.LoginUser))
		resp := ts.MakeRequest(t, req)

		// Assert error response
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Equal(t, "Invalid email or password.", resp.Body["message"])
	})

	t.Run("jwt generation failure", func(t *testing.T) {
		// Create test user with unique email
		testUser := testutil.NewTestUser()
		testUser.Email = "failure@example.com"
		testDB.CreateTestUser(t, testUser)

		// Set up mock to return error
		mockJWT.ExpectGenerateToken(testUser.ID, "", errors.New("jwt generation failed"))

		// Make request
		loginReq := api.LoginRequest{
			Email: types.Email(testUser.Email),
		}

		req := testutil.Request{
			Method: "POST",
			Path:   "/login",
			Body:   loginReq,
		}

		ts := testutil.NewTestServer(t, http.HandlerFunc(server.LoginUser))
		resp := ts.MakeRequest(t, req)

		// Assert error response
		assert.Equal(t, http.StatusInternalServerError, resp.Code)
		assert.Equal(t, "An unexpected error occurred.", resp.Body["message"])

		// Verify mock was called
		mockJWT.AssertExpectations(t)
	})
}

// TODO: Add tests for PingProtected endpoint
