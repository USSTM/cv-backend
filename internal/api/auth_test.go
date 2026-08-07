package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	appmiddleware "github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/go-chi/chi/v5"
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
		require.IsType(t, verifyOTPCookieResponse{}, response)
		resp := response.(verifyOTPCookieResponse).VerifyOTP200JSONResponse
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEmpty(t, resp.RefreshToken)
		recorder := httptest.NewRecorder()
		require.NoError(t, response.(verifyOTPCookieResponse).VisitVerifyOTPResponse(recorder))
		assertSessionCookies(t, recorder.Result().Cookies())
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
		require.IsType(t, refreshTokenCookieResponse{}, response)
		resp := response.(refreshTokenCookieResponse).RefreshToken200JSONResponse
		assert.NotEmpty(t, resp.AccessToken)
		assert.NotEqual(t, refreshToken, resp.RefreshToken)
		recorder := httptest.NewRecorder()
		require.NoError(t, response.(refreshTokenCookieResponse).VisitRefreshTokenResponse(recorder))
		assertSessionCookies(t, recorder.Result().Cookies())
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
		require.IsType(t, logoutCookieResponse{}, response)
		recorder := httptest.NewRecorder()
		require.NoError(t, response.(logoutCookieResponse).VisitLogoutResponse(recorder))
		assertExpiredSessionCookies(t, recorder.Result().Cookies())
	})

	t.Run("nil body", func(t *testing.T) {
		server, _, _, _ := newAuthTestServer(t)

		response, err := server.Logout(context.Background(), api.LogoutRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.Logout400JSONResponse{}, response)
	})
}

func TestServer_CookieOnlyRefreshAndLogout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, _, authSvc := newAuthTestServer(t)
	user := testDB.NewUser(t).WithEmail("cookie-session@example.com").Create()
	code, err := authSvc.RequestOTP(context.Background(), user.Email)
	require.NoError(t, err)
	_, refreshToken, err := authSvc.VerifyOTP(context.Background(), user.Email, code)
	require.NoError(t, err)

	handler := cookieAuthHandler(server)
	refreshRequest := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	refreshRequest.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: refreshToken})
	refreshRecorder := httptest.NewRecorder()
	handler.ServeHTTP(refreshRecorder, refreshRequest)
	require.Equal(t, http.StatusOK, refreshRecorder.Code)
	assertSessionCookies(t, refreshRecorder.Result().Cookies())

	rotatedRefresh := cookieValue(refreshRecorder.Result().Cookies(), refreshTokenCookieName)
	require.NotEmpty(t, rotatedRefresh)
	require.NotEqual(t, refreshToken, rotatedRefresh)

	logoutRequest := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutRequest.AddCookie(&http.Cookie{Name: refreshTokenCookieName, Value: rotatedRefresh})
	logoutRecorder := httptest.NewRecorder()
	handler.ServeHTTP(logoutRecorder, logoutRequest)
	require.Equal(t, http.StatusOK, logoutRecorder.Code)
	assertExpiredSessionCookies(t, logoutRecorder.Result().Cookies())
}

func TestServer_AuthCookiesRespectSecureConfiguration(t *testing.T) {
	server := Server{cookies: CookieConfig{
		AccessExpiry:  time.Minute,
		RefreshExpiry: time.Hour,
		Secure:        true,
	}}

	cookies := server.authCookies("access", "refresh")
	require.Len(t, cookies, 2)
	for _, cookie := range cookies {
		assert.True(t, cookie.Secure)
		assert.True(t, cookie.HttpOnly)
		assert.Equal(t, "/", cookie.Path)
		assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
		assert.Positive(t, cookie.MaxAge)
		assert.True(t, cookie.Expires.After(time.Now()))
	}
}

func cookieAuthHandler(server *Server) http.Handler {
	router := chi.NewRouter()
	router.Use(appmiddleware.AuthCookieContext)
	api.HandlerFromMux(api.NewStrictHandler(server, nil), router)
	return router
}

func assertSessionCookies(t *testing.T, cookies []*http.Cookie) {
	t.Helper()
	require.Len(t, cookies, 2)
	assert.Equal(t, accessTokenCookieName, cookies[0].Name)
	assert.Equal(t, refreshTokenCookieName, cookies[1].Name)
	for _, cookie := range cookies {
		assert.NotEmpty(t, cookie.Value)
		assert.True(t, cookie.HttpOnly)
		assert.Equal(t, "/", cookie.Path)
		assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
		assert.Positive(t, cookie.MaxAge)
	}
}

func assertExpiredSessionCookies(t *testing.T, cookies []*http.Cookie) {
	t.Helper()
	require.Len(t, cookies, 2)
	for _, cookie := range cookies {
		assert.True(t, cookie.HttpOnly)
		assert.Empty(t, cookie.Value)
		assert.Less(t, cookie.MaxAge, 0)
	}
}

func cookieValue(cookies []*http.Cookie, name string) string {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
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
