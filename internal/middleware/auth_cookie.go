package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type refreshTokenContextKey struct{}

const refreshTokenCookieName = "refresh_token"

// // AuthCookieContext makes the HTTP-only refresh cookie available to strict
// // handlers, whose generated request objects contain only decoded JSON bodies.
func AuthCookieContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var refreshToken string
		if cookie, err := r.Cookie(refreshTokenCookieName); err == nil && cookie.Value != "" {
			refreshToken = cookie.Value
			ctx := context.WithValue(r.Context(), refreshTokenContextKey{}, cookie.Value)
			r = r.WithContext(ctx)
		}
		// Refresh/logout requests retain their JSON API contract. When the
		// browser sends only the HTTP-only cookie, provide the equivalent body
		// internally before OpenAPI validation and strict decoding.
		if (r.URL.Path == "/auth/refresh" || r.URL.Path == "/auth/logout") && r.ContentLength <= 0 && refreshToken != "" {
			body, _ := json.Marshal(struct {
				RefreshToken string `json:"refresh_token"`
			}{RefreshToken: refreshToken})
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
			r.Header.Set("Content-Type", "application/json")
		}
		next.ServeHTTP(w, r)
	})
}

func GetRefreshTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(refreshTokenContextKey{}).(string)
	return token
}
