package api

import (
	"context"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_HealthCheck(t *testing.T) {
	server, _, _ := newTestServer(t)

	t.Run("returns 200 OK with timestamp", func(t *testing.T) {
		request := api.HealthCheckRequestObject{}

		response, err := server.HealthCheck(context.Background(), request)

		require.NoError(t, err)
		require.IsType(t, api.HealthCheck200JSONResponse{}, response)

		healthResp := response.(api.HealthCheck200JSONResponse)
		assert.Equal(t, "ok", healthResp.Status)
		assert.WithinDuration(t, time.Now(), healthResp.Timestamp, 1*time.Second)
	})

	t.Run("works without authentication", func(t *testing.T) {
		// without authenticated user
		ctx := context.Background()
		request := api.HealthCheckRequestObject{}

		response, err := server.HealthCheck(ctx, request)

		require.NoError(t, err)
		require.IsType(t, api.HealthCheck200JSONResponse{}, response)
	})
}

func TestServer_ReadinessCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, _, _ := newTestServer(t)

	t.Run("returns 200 ready when database is healthy", func(t *testing.T) {
		request := api.ReadinessCheckRequestObject{}

		response, err := server.ReadinessCheck(context.Background(), request)

		require.NoError(t, err)
		require.IsType(t, api.ReadinessCheck200JSONResponse{}, response)

		readyResp := response.(api.ReadinessCheck200JSONResponse)
		assert.Equal(t, "ready", string(readyResp.Status))
		assert.WithinDuration(t, time.Now(), readyResp.Timestamp, 1*time.Second)
		require.NotNil(t, readyResp.Checks)
		assert.Equal(t, "ok", readyResp.Checks["database"])
	})

	t.Run("works without authentication", func(t *testing.T) {
		// without authenticated user
		ctx := context.Background()
		request := api.ReadinessCheckRequestObject{}

		response, err := server.ReadinessCheck(ctx, request)

		require.NoError(t, err)
		// success (we have healthy DB)
		require.IsType(t, api.ReadinessCheck200JSONResponse{}, response)
	})
}
