package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/logging"
	"github.com/google/uuid"
)

type contextKey string

const (
	requestIDKey contextKey = "requestID"
	userIDKey    contextKey = "userID"
	loggerKey    contextKey = "logger"
)

// middleware adds request ID, user ID, and IP address to context
func RequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// request ID
		requestID := uuid.New().String()
		ctx = context.WithValue(ctx, requestIDKey, requestID)

		// user ID from JWT
		userID := ""
		if user, ok := auth.GetAuthenticatedUser(ctx); ok {
			userID = user.Email
		}
		if userID != "" {
			ctx = context.WithValue(ctx, userIDKey, userID)
		}

		// client IP
		clientIP := getClientIP(r)

		// Create logger with request context
		logger := logging.With(
			"request_id", requestID,
			"user_id", userID,
			"client_ip", clientIP,
		)
		ctx = context.WithValue(ctx, loggerKey, logger)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetLoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	// Fallback to default logger if not found
	return slog.Default()
}

func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// attempt to get client IP, later can be used for rate limiting
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header for proxied requests
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// take the first one
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to remote address
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}
