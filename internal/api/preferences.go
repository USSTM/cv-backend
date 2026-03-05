package api

import (
	"context"
	"encoding/json"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/preferences"
)

func (s Server) GetMyPreferences(ctx context.Context, _ genapi.GetMyPreferencesRequestObject) (genapi.GetMyPreferencesResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.GetMyPreferences401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	stored, err := s.db.Queries().GetUserPreferences(ctx, user.ID)
	if err != nil {
		middleware.GetLoggerFromContext(ctx).Error("failed to get user preferences", "user_id", user.ID, "error", err)
		return genapi.GetMyPreferences500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	prefs, err := preferences.Merge(stored)
	if err != nil {
		middleware.GetLoggerFromContext(ctx).Error("failed to parse user preferences", "user_id", user.ID, "error", err)
		return genapi.GetMyPreferences500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	return genapi.GetMyPreferences200JSONResponse{
		EmailNotifications: prefs.EmailNotifications,
	}, nil
}

func (s Server) UpdateMyPreferences(ctx context.Context, request genapi.UpdateMyPreferencesRequestObject) (genapi.UpdateMyPreferencesResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.UpdateMyPreferences401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	if request.Body == nil {
		return genapi.UpdateMyPreferences400JSONResponse(ValidationErr("Request body is required", nil).Create()), nil
	}

	stored, err := s.db.Queries().GetUserPreferences(ctx, user.ID)
	if err != nil {
		logger.Error("failed to get user preferences", "user_id", user.ID, "error", err)
		return genapi.UpdateMyPreferences500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	current, err := preferences.Merge(stored)
	if err != nil {
		logger.Error("failed to parse user preferences", "user_id", user.ID, "error", err)
		return genapi.UpdateMyPreferences500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	if request.Body.EmailNotifications != nil {
		current.EmailNotifications = *request.Body.EmailNotifications
	}

	raw, err := json.Marshal(current)
	if err != nil {
		logger.Error("failed to marshal preferences", "user_id", user.ID, "error", err)
		return genapi.UpdateMyPreferences500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	updated, err := s.db.Queries().UpdateUserPreferences(ctx, db.UpdateUserPreferencesParams{
		Preferences: raw,
		ID:          user.ID,
	})
	if err != nil {
		logger.Error("failed to update preferences", "user_id", user.ID, "error", err)
		return genapi.UpdateMyPreferences500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	result, err := preferences.Merge(updated)
	if err != nil {
		logger.Error("failed to parse updated preferences", "user_id", user.ID, "error", err)
		return genapi.UpdateMyPreferences500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	return genapi.UpdateMyPreferences200JSONResponse{
		EmailNotifications: result.EmailNotifications,
	}, nil
}
