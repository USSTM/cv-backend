package api

import (
	"context"
	"testing"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/preferences"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_GetMyPreferences(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, _ := newTestServer(t)

	t.Run("new user gets default set", func(t *testing.T) {
		user := testDB.NewUser(t).WithEmail("prefs@default.ca").AsMember().Create()
		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		resp, err := server.GetMyPreferences(ctx, api.GetMyPreferencesRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.GetMyPreferences200JSONResponse{}, resp)

		prefs := resp.(api.GetMyPreferences200JSONResponse)
		assert.True(t, prefs.EmailNotifications, "email_notifications should default to true")
	})

	t.Run("returns stored value when set", func(t *testing.T) {
		user := testDB.NewUser(t).
			WithEmail("prefs@stored.ca").
			AsMember().
			WithPreferences(preferences.UserPreferences{EmailNotifications: false}).
			Create()
		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		resp, err := server.GetMyPreferences(ctx, api.GetMyPreferencesRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.GetMyPreferences200JSONResponse{}, resp)

		prefs := resp.(api.GetMyPreferences200JSONResponse)
		assert.False(t, prefs.EmailNotifications)
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		resp, err := server.GetMyPreferences(context.Background(), api.GetMyPreferencesRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.GetMyPreferences401JSONResponse{}, resp)
	})
}

func TestServer_UpdateMyPreferences(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, _ := newTestServer(t)

	t.Run("opt out of email notifications", func(t *testing.T) {
		user := testDB.NewUser(t).WithEmail("prefs@optout.ca").AsMember().Create()
		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		disabled := false
		resp, err := server.UpdateMyPreferences(ctx, api.UpdateMyPreferencesRequestObject{
			Body: &api.UserPreferencesUpdate{EmailNotifications: &disabled},
		})
		require.NoError(t, err)
		require.IsType(t, api.UpdateMyPreferences200JSONResponse{}, resp)

		prefs := resp.(api.UpdateMyPreferences200JSONResponse)
		assert.False(t, prefs.EmailNotifications)
	})

	t.Run("unset fields not overwritten", func(t *testing.T) {
		user := testDB.NewUser(t).
			WithEmail("prefs@partial.ca").
			AsMember().
			WithPreferences(preferences.UserPreferences{EmailNotifications: false}).
			Create()
		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		resp, err := server.UpdateMyPreferences(ctx, api.UpdateMyPreferencesRequestObject{
			Body: &api.UserPreferencesUpdate{},
		})
		require.NoError(t, err)
		require.IsType(t, api.UpdateMyPreferences200JSONResponse{}, resp)

		prefs := resp.(api.UpdateMyPreferences200JSONResponse)
		assert.False(t, prefs.EmailNotifications, "existing false value should be preserved when not patched")
	})

	t.Run("unauthenticated returns 401", func(t *testing.T) {
		resp, err := server.UpdateMyPreferences(context.Background(), api.UpdateMyPreferencesRequestObject{
			Body: &api.UserPreferencesUpdate{},
		})
		require.NoError(t, err)
		require.IsType(t, api.UpdateMyPreferences401JSONResponse{}, resp)
	})
}
