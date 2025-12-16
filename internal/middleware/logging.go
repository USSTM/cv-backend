package middleware

import (
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// logs HTTP requests and responses with logger module
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			written:        false,
		}

		// Get logger
		logger := GetLoggerFromContext(r.Context())

		// incoming request
		logger.Info("Request received",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery)

		// Call next handler
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Determine log level based on status code
		statusCode := wrapped.statusCode
		logAttrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", statusCode,
			"duration_ms", duration.Milliseconds(),
		}

		switch {
		case statusCode >= 500:
			// (5xx) ERROR level
			logger.Error("Request completed with server error", logAttrs...)
		case statusCode >= 400:
			// (4xx) WARN level
			logger.Warn("Request completed with client error", logAttrs...)
		default:
			// (2xx, 3xx) INFO level
			logger.Info("Request completed successfully", logAttrs...)
		}
	})
}
