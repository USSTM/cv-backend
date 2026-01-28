package api

import (
	"context"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/jackc/pgx/v5/pgtype"
)

// pgtype.Time to HH:MM:SS string
func formatPgTime(t pgtype.Time) string {
	val, err := t.Value()
	if err != nil || val == nil {
		return "00:00:00"
	}
	// Value() format "HH:MM:SS.microseconds"
	str := val.(string)
	if len(str) >= 8 {
		return str[:8] // Return just HH:MM:SS
	}
	return str
}

func (s Server) ListTimeSlots(ctx context.Context, request api.ListTimeSlotsRequestObject) (api.ListTimeSlotsResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	_, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.ListTimeSlots401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	timeSlots, err := s.db.Queries().ListTimeSlots(ctx)
	if err != nil {
		logger.Error("Failed to list time slots", "error", err)
		return api.ListTimeSlots500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	response := make(api.ListTimeSlots200JSONResponse, 0, len(timeSlots))
	for _, ts := range timeSlots {
		response = append(response, api.TimeSlot{
			Id:        ts.ID,
			StartTime: formatPgTime(ts.StartTime),
			EndTime:   formatPgTime(ts.EndTime),
		})
	}

	return response, nil
}
