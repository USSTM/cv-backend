package api

import (
	"context"
	"testing"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_RequestOTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("success", func(t *testing.T) {
		server, testDB, _, _ := newAuthTestServer(t)

		user := testDB.NewUser(t).WithEmail("otp-request@example.com").Create()
		response, err := server.RequestOTP(context.Background(), api.RequestOTPRequestObject{
			Body: &api.RequestOTPJSONRequestBody{Email: types.Email(user.Email)},
		})

		require.NoError(t, err)
		require.IsType(t, api.RequestOTP200JSONResponse{}, response)
	})

	t.Run("user not found", func(t *testing.T) {
		server, _, _, _ := newAuthTestServer(t)

		response, err := server.RequestOTP(context.Background(), api.RequestOTPRequestObject{
			Body: &api.RequestOTPJSONRequestBody{Email: "nobody@example.com"},
		})

		require.NoError(t, err)
		require.IsType(t, api.RequestOTP200JSONResponse{}, response)
	})

	t.Run("cooldown", func(t *testing.T) {
		server, testDB, _, _ := newAuthTestServer(t)

		user := testDB.NewUser(t).WithEmail("otp-cooldown@example.com").Create()

		// first request sets cooldown
		_, err := server.RequestOTP(context.Background(), api.RequestOTPRequestObject{
			Body: &api.RequestOTPJSONRequestBody{Email: types.Email(user.Email)},
		})
		require.NoError(t, err)

		// second request hits cooldown
		response, err := server.RequestOTP(context.Background(), api.RequestOTPRequestObject{
			Body: &api.RequestOTPJSONRequestBody{Email: types.Email(user.Email)},
		})
		require.NoError(t, err)
		require.IsType(t, api.RequestOTP429JSONResponse{}, response)
	})

	t.Run("nil body", func(t *testing.T) {
		server, _, _, _ := newAuthTestServer(t)

		response, err := server.RequestOTP(context.Background(), api.RequestOTPRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.RequestOTP400JSONResponse{}, response)
	})
}

func TestServer_VerifyOTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("success", func(t *testing.T) {
		server, testDB, _, authSvc := newAuthTestServer(t)

		user := testDB.NewUser(t).WithEmail("verify@example.com").Create()
		code, err := authSvc.RequestOTP(context.Background(), user.Email)
		require.NoError(t, err)

		response, err := server.VerifyOTP(context.Background(), api.VerifyOTPRequestObject{
			Body: &api.VerifyOTPJSONRequestBody{
				Email: types.Email(user.Email),
				Code:  code,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.VerifyOTP200JSONResponse{}, response)
		resp := response.(api.VerifyOTP200JSONResponse)
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEmpty(t, resp.RefreshToken)
	})

	t.Run("invalid code", func(t *testing.T) {
		server, testDB, _, authSvc := newAuthTestServer(t)

		user := testDB.NewUser(t).WithEmail("invalid@example.com").Create()
		_, err := authSvc.RequestOTP(context.Background(), user.Email)
		require.NoError(t, err)

		response, err := server.VerifyOTP(context.Background(), api.VerifyOTPRequestObject{
			Body: &api.VerifyOTPJSONRequestBody{
				Email: types.Email(user.Email),
				Code:  "000000",
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.VerifyOTP400JSONResponse{}, response)
	})
}

func TestServer_RefreshToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("success", func(t *testing.T) {
		server, testDB, _, authSvc := newAuthTestServer(t)

		user := testDB.NewUser(t).WithEmail("refresh@example.com").Create()
		code, err := authSvc.RequestOTP(context.Background(), user.Email)
		require.NoError(t, err)
		_, refreshToken, err := authSvc.VerifyOTP(context.Background(), user.Email, code)
		require.NoError(t, err)

		response, err := server.RefreshToken(context.Background(), api.RefreshTokenRequestObject{
			Body: &api.RefreshTokenJSONRequestBody{RefreshToken: refreshToken},
		})

		require.NoError(t, err)
		require.IsType(t, api.RefreshToken200JSONResponse{}, response)
		resp := response.(api.RefreshToken200JSONResponse)
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEqual(t, refreshToken, resp.RefreshToken)
	})

	t.Run("invalid token", func(t *testing.T) {
		server, _, _, _ := newAuthTestServer(t)

		response, err := server.RefreshToken(context.Background(), api.RefreshTokenRequestObject{
			Body: &api.RefreshTokenJSONRequestBody{RefreshToken: "not-a-real-token"},
		})

		require.NoError(t, err)
		require.IsType(t, api.RefreshToken401JSONResponse{}, response)
	})
}

func TestServer_Logout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("success", func(t *testing.T) {
		server, testDB, _, authSvc := newAuthTestServer(t)

		user := testDB.NewUser(t).WithEmail("logout@example.com").Create()
		code, err := authSvc.RequestOTP(context.Background(), user.Email)
		require.NoError(t, err)
		_, refreshToken, err := authSvc.VerifyOTP(context.Background(), user.Email, code)
		require.NoError(t, err)

		response, err := server.Logout(context.Background(), api.LogoutRequestObject{
			Body: &api.LogoutJSONRequestBody{RefreshToken: refreshToken},
		})

		require.NoError(t, err)
		require.IsType(t, api.Logout200JSONResponse{}, response)
	})

	t.Run("nil body", func(t *testing.T) {
		server, _, _, _ := newAuthTestServer(t)

		response, err := server.Logout(context.Background(), api.LogoutRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.Logout400JSONResponse{}, response)
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

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())
		response, err := server.PingProtected(ctx, api.PingProtectedRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.PingProtected200JSONResponse{}, response)
		pingResp := response.(api.PingProtected200JSONResponse)
		assert.Contains(t, pingResp.Message, "PONG! Hello")
		assert.Contains(t, pingResp.Message, testUser.Email)
		assert.NotZero(t, pingResp.Timestamp)
	})

	t.Run("unauthorized user", func(t *testing.T) {
		response, err := server.PingProtected(context.Background(), api.PingProtectedRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.PingProtected401JSONResponse{}, response)
		errorResp := response.(api.PingProtected401JSONResponse)
		assert.Equal(t, "AUTHENTICATION_REQUIRED", string(errorResp.Error.Code))
	})

	t.Run("insufficient permissions", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("test2@example.com").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, false, nil)

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())
		response, err := server.PingProtected(ctx, api.PingProtectedRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.PingProtected401JSONResponse{}, response)
		errorResp := response.(api.PingProtected401JSONResponse)
		assert.Equal(t, "PERMISSION_DENIED", string(errorResp.Error.Code))
	})
}

func TestServer_Logout_RevokesToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, _, authSvc := newAuthTestServer(t)

	user := testDB.NewUser(t).WithEmail("revoke@example.com").Create()
	code, err := authSvc.RequestOTP(context.Background(), user.Email)
	require.NoError(t, err)
	_, refreshToken, err := authSvc.VerifyOTP(context.Background(), user.Email, code)
	require.NoError(t, err)

	_, err = server.Logout(context.Background(), api.LogoutRequestObject{
		Body: &api.LogoutJSONRequestBody{RefreshToken: refreshToken},
	})
	require.NoError(t, err)

	response, err := server.RefreshToken(context.Background(), api.RefreshTokenRequestObject{
		Body: &api.RefreshTokenJSONRequestBody{RefreshToken: refreshToken},
	})
	require.NoError(t, err)
	require.IsType(t, api.RefreshToken401JSONResponse{}, response)
}

func TestServer_Refresh_RotatesToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, _, authSvc := newAuthTestServer(t)

	user := testDB.NewUser(t).WithEmail("rotate@example.com").Create()
	code, err := authSvc.RequestOTP(context.Background(), user.Email)
	require.NoError(t, err)
	_, oldRefresh, err := authSvc.VerifyOTP(context.Background(), user.Email, code)
	require.NoError(t, err)

	_, err = server.RefreshToken(context.Background(), api.RefreshTokenRequestObject{
		Body: &api.RefreshTokenJSONRequestBody{RefreshToken: oldRefresh},
	})
	require.NoError(t, err)

	response, err := server.RefreshToken(context.Background(), api.RefreshTokenRequestObject{
		Body: &api.RefreshTokenJSONRequestBody{RefreshToken: oldRefresh},
	})
	require.NoError(t, err)
	require.IsType(t, api.RefreshToken401JSONResponse{}, response)
}
