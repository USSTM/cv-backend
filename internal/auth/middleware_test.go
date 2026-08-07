package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccessTokenUsesHTTPOnlyCookieWhenBearerHeaderIsAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.AddCookie(&http.Cookie{Name: accessTokenCookieName, Value: "cookie-access-token", HttpOnly: true})

	token, err := accessToken(req)
	require.NoError(t, err)
	assert.Equal(t, "cookie-access-token", token)
}

func TestAccessTokenPrefersBearerHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Authorization", "Bearer header-access-token")
	req.AddCookie(&http.Cookie{Name: accessTokenCookieName, Value: "cookie-access-token", HttpOnly: true})

	token, err := accessToken(req)
	require.NoError(t, err)
	assert.Equal(t, "header-access-token", token)
}
