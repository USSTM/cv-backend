package api

import (
	"context"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/middleware"
)

func (s Server) HealthCheck(ctx context.Context, request api.HealthCheckRequestObject) (api.HealthCheckResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)
	logger.Debug("Health check requested")

	return api.HealthCheck200JSONResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
	}, nil
}

// Returns 200 if ready, 503 if not ready.
func (s Server) ReadinessCheck(ctx context.Context, request api.ReadinessCheckRequestObject) (api.ReadinessCheckResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)
	logger.Debug("Readiness check requested")

	checks := make(map[string]string)

	// Check database connectivity
	dbCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := s.db.Pool().Ping(dbCtx); err != nil {
		logger.Warn("Database health check failed", "error", err)
		checks["database"] = "failed: " + err.Error()

		return api.ReadinessCheck503JSONResponse{
			Status:    "not_ready",
			Timestamp: time.Now().UTC(),
			Checks:    checks,
		}, nil
	}

	checks["database"] = "ok"
	logger.Debug("Readiness check passed")

	return api.ReadinessCheck200JSONResponse{
		Status:    "ready",
		Timestamp: time.Now().UTC(),
		Checks:    checks,
	}, nil
}
