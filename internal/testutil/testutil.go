package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestServer provides a test server setup for API testing
type TestServer struct {
	*httptest.Server
	DB       *TestDatabase
	MockJWT  *MockJWTService
	MockAuth *MockAuthenticator
}

// NewTestServer creates a test server with real database and service mocks
func NewTestServer(t *testing.T, handler http.Handler) *TestServer {
	testDB := NewTestDatabase(t)
	mockJWT := NewMockJWTService(t)
	mockAuth := NewMockAuthenticator(t)

	server := httptest.NewServer(handler)
	return &TestServer{
		Server:   server,
		DB:       testDB,
		MockJWT:  mockJWT,
		MockAuth: mockAuth,
	}
}

// Request represents a test HTTP request
type Request struct {
	Method      string
	Path        string
	Body        interface{}
	Headers     map[string]string
	QueryParams map[string]string
}

// Response represents a test HTTP response
type Response struct {
	*httptest.ResponseRecorder
	Body map[string]interface{}
}

// MakeRequest creates and executes a test HTTP request
func (ts *TestServer) MakeRequest(t *testing.T, req Request) *Response {
	var bodyReader *bytes.Reader

	if req.Body != nil {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	var httpReq *http.Request
	var err error

	if bodyReader != nil {
		httpReq, err = http.NewRequest(req.Method, req.Path, bodyReader)
	} else {
		httpReq, err = http.NewRequest(req.Method, req.Path, nil)
	}

	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Set headers
	if req.Headers != nil {
		for key, value := range req.Headers {
			httpReq.Header.Set(key, value)
		}
	}

	// Set query parameters
	if req.QueryParams != nil {
		q := httpReq.URL.Query()
		for key, value := range req.QueryParams {
			q.Add(key, value)
		}
		httpReq.URL.RawQuery = q.Encode()
	}

	// Set default content type for JSON requests
	if req.Body != nil && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()

	// Make request directly to the handler
	ts.Server.Config.Handler.ServeHTTP(recorder, httpReq)

	// Parse response body
	var responseBody map[string]interface{}
	if recorder.Body.Len() > 0 {
		decoder := json.NewDecoder(recorder.Body)
		if err := decoder.Decode(&responseBody); err != nil {
			t.Logf("Failed to decode response body: %v", err)
		}
	}

	return &Response{
		ResponseRecorder: recorder,
		Body:             responseBody,
	}
}

// AuthenticatedRequest creates a request with authentication headers
func (ts *TestServer) AuthenticatedRequest(t *testing.T, req Request, token string) *Response {
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	req.Headers["Authorization"] = "Bearer " + token
	return ts.MakeRequest(t, req)
}

// TestUser represents a test user for authentication
type TestUser struct {
	ID          uuid.UUID
	Email       string
	Permissions []string
	Roles       []string
}

// NewTestUser creates a test user with default values
func NewTestUser() *TestUser {
	return &TestUser{
		ID:          uuid.New(),
		Email:       "test@example.com",
		Permissions: []string{"view_own_data"},
		Roles:       []string{"member"}, // Use the actual role from the migration
	}
}

// WithPermissions adds permissions to the test user
func (u *TestUser) WithPermissions(permissions ...string) *TestUser {
	u.Permissions = append(u.Permissions, permissions...)
	return u
}

// WithRoles adds roles to the test user
func (u *TestUser) WithRoles(roles ...string) *TestUser {
	u.Roles = append(u.Roles, roles...)
	return u
}

// ToAuthenticatedUser converts TestUser to auth.AuthenticatedUser
func (u *TestUser) ToAuthenticatedUser() *auth.AuthenticatedUser {
	permissions := make([]db.GetUserPermissionsRow, len(u.Permissions))
	for i, perm := range u.Permissions {
		permissions[i] = db.GetUserPermissionsRow{
			Name: perm,
		}
	}

	roles := make([]db.GetUserRolesRow, len(u.Roles))
	for i, role := range u.Roles {
		roles[i] = db.GetUserRolesRow{
			RoleName: pgtype.Text{String: role, Valid: true},
		}
	}

	return &auth.AuthenticatedUser{
		ID:          u.ID,
		Email:       u.Email,
		Permissions: permissions,
		Roles:       roles,
	}
}

// ContextWithUser adds a test user to the context
func ContextWithUser(ctx context.Context, user *TestUser) context.Context {
	ctx = context.WithValue(ctx, auth.UserIDKey, user.ID)
	ctx = context.WithValue(ctx, auth.UserClaimsKey, user.ToAuthenticatedUser())
	return ctx
}

// TimeNow returns a consistent time for testing
func TimeNow() time.Time {
	return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
}

// NewUUID returns a deterministic UUID for testing
func NewUUID() uuid.UUID {
	return uuid.MustParse("12345678-1234-5678-9012-123456789012")
}

// AssertJSON checks if the response body contains expected JSON fields
func AssertJSON(t *testing.T, resp *Response, field string, expected interface{}) {
	if resp.Body[field] != expected {
		t.Errorf("Expected %s to be %v, got %v", field, expected, resp.Body[field])
	}
}

// AssertJSONExists checks if a JSON field exists in the response
func AssertJSONExists(t *testing.T, resp *Response, field string) {
	if _, exists := resp.Body[field]; !exists {
		t.Errorf("Expected field %s to exist in response", field)
	}
}
