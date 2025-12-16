package api

import (
	"github.com/USSTM/cv-backend/internal/rbac"
	"context"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// approvers set availability schedule
func (s Server) CreateAvailability(ctx context.Context, request api.CreateAvailabilityRequestObject) (api.CreateAvailabilityResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.CreateAvailability401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	// manage time slots (approvers only)
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageTimeSlots, nil)
	if err != nil {
		logger.Error("Failed to check permission", "error", err)
		return api.CreateAvailability500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	if !hasPermission {
		return api.CreateAvailability403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// future date?
	date := request.Body.Date.Time
	if date.Before(time.Now().Truncate(24 * time.Hour)) {
		return api.CreateAvailability400JSONResponse{
			Code:    400,
			Message: "Date must be in the future",
		}, nil
	}

	// conflict with availability in bookings?
	hasConflict, err := s.db.Queries().CheckAvailabilityConflict(ctx, db.CheckAvailabilityConflictParams{
		UserID:     &user.ID,
		TimeSlotID: &request.Body.TimeSlotId,
		Date: pgtype.Date{
			Time:  date,
			Valid: true,
		},
	})
	if err != nil {
		logger.Error("Failed to check availability conflict", "error", err)
		return api.CreateAvailability500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	if hasConflict {
		return api.CreateAvailability409JSONResponse{
			Code:    409,
			Message: "You already have availability set for this time slot on this date",
		}, nil
	}

	// create
	availability, err := s.db.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
		ID:         uuid.New(),
		UserID:     &user.ID,
		TimeSlotID: &request.Body.TimeSlotId,
		Date: pgtype.Date{
			Time:  date,
			Valid: true,
		},
	})
	if err != nil {
		logger.Error("Failed to create availability", "error", err)
		return api.CreateAvailability500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	// fetch time slot
	timeSlot, err := s.db.Queries().GetTimeSlotByID(ctx, *availability.TimeSlotID)
	if err != nil {
		logger.Error("Failed to fetch time slot", "error", err)
		return api.CreateAvailability500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	return api.CreateAvailability201JSONResponse{
		Id:         availability.ID,
		UserId:     *availability.UserID,
		TimeSlotId: *availability.TimeSlotID,
		Date:       openapi_types.Date{Time: availability.Date.Time},
		StartTime:  formatPgTime(timeSlot.StartTime),
		EndTime:    formatPgTime(timeSlot.EndTime),
	}, nil
}

// filter availability
func (s Server) ListAvailability(ctx context.Context, request api.ListAvailabilityRequestObject) (api.ListAvailabilityResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	_, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.ListAvailability401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	// date filter
	var dateParam pgtype.Date
	if request.Params.Date != nil {
		dateParam = pgtype.Date{Time: request.Params.Date.Time, Valid: true}
	}

	// user_id filter
	var userIDParam *uuid.UUID
	if request.Params.UserId != nil {
		uid := uuid.UUID(*request.Params.UserId)
		userIDParam = &uid
	}

	availabilities, err := s.db.Queries().ListAvailability(ctx, db.ListAvailabilityParams{
		FilterDate:   dateParam,
		FilterUserID: userIDParam,
	})
	if err != nil {
		logger.Error("Failed to list availability", "error", err)
		return api.ListAvailability500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	// format to openapi spec
	response := make(api.ListAvailability200JSONResponse, 0, len(availabilities))
	for _, a := range availabilities {
		response = append(response, api.AvailabilityResponse{
			Id:         a.ID,
			UserId:     *a.UserID,
			TimeSlotId: *a.TimeSlotID,
			Date:       openapi_types.Date{Time: a.Date.Time},
			UserEmail:  openapi_types.Email(a.UserEmail),
			StartTime:  formatPgTime(a.StartTime),
			EndTime:    formatPgTime(a.EndTime),
		})
	}

	return response, nil
}

// returns approvers available on date
func (s Server) GetAvailabilityByDate(ctx context.Context, request api.GetAvailabilityByDateRequestObject) (api.GetAvailabilityByDateResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	_, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetAvailabilityByDate401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	date := request.Date.Time

	availabilities, err := s.db.Queries().GetAvailabilityByDate(ctx, pgtype.Date{
		Time:  date,
		Valid: true,
	})
	if err != nil {
		logger.Error("Failed to fetch availability by date", "error", err)
		return api.GetAvailabilityByDate500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	response := make(api.GetAvailabilityByDate200JSONResponse, 0, len(availabilities))
	for _, a := range availabilities {
		response = append(response, api.AvailabilityResponse{
			Id:         a.ID,
			UserId:     *a.UserID,
			TimeSlotId: *a.TimeSlotID,
			Date:       openapi_types.Date{Time: a.Date.Time},
			UserEmail:  openapi_types.Email(a.UserEmail),
			StartTime:  formatPgTime(a.StartTime),
			EndTime:    formatPgTime(a.EndTime),
		})
	}

	return response, nil
}

func (s Server) GetAvailabilityByID(ctx context.Context, request api.GetAvailabilityByIDRequestObject) (api.GetAvailabilityByIDResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	_, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetAvailabilityByID401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	availability, err := s.db.Queries().GetAvailabilityByID(ctx, request.Id)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return api.GetAvailabilityByID404JSONResponse{
				Code:    404,
				Message: "Availability not found",
			}, nil
		}
		logger.Error("Failed to fetch availability", "error", err)
		return api.GetAvailabilityByID500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	return api.GetAvailabilityByID200JSONResponse{
		Id:         availability.ID,
		UserId:     *availability.UserID,
		TimeSlotId: *availability.TimeSlotID,
		Date:       openapi_types.Date{Time: availability.Date.Time},
		UserEmail:  openapi_types.Email(availability.UserEmail),
		StartTime:  formatPgTime(availability.StartTime),
		EndTime:    formatPgTime(availability.EndTime),
	}, nil
}

// returns user's schedule
func (s Server) GetUserAvailability(ctx context.Context, request api.GetUserAvailabilityRequestObject) (api.GetUserAvailabilityResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	_, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUserAvailability401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	// optional date range
	var fromDate, toDate pgtype.Date
	if request.Params.FromDate != nil {
		fromDate = pgtype.Date{Time: request.Params.FromDate.Time, Valid: true}
	}

	if request.Params.ToDate != nil {
		toDate = pgtype.Date{Time: request.Params.ToDate.Time, Valid: true}
	}

	availabilities, err := s.db.Queries().GetUserAvailability(ctx, db.GetUserAvailabilityParams{
		UserID:   &request.UserId,
		FromDate: fromDate,
		ToDate:   toDate,
	})
	if err != nil {
		logger.Error("Failed to fetch user availability", "error", err)
		return api.GetUserAvailability500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	response := make(api.GetUserAvailability200JSONResponse, 0, len(availabilities))
	for _, a := range availabilities {
		response = append(response, api.UserAvailabilityResponse{
			Id:         a.ID,
			UserId:     *a.UserID,
			TimeSlotId: *a.TimeSlotID,
			Date:       openapi_types.Date{Time: a.Date.Time},
			StartTime:  formatPgTime(a.StartTime),
			EndTime:    formatPgTime(a.EndTime),
		})
	}

	return response, nil
}

// removes an availability entry
func (s Server) DeleteAvailability(ctx context.Context, request api.DeleteAvailabilityRequestObject) (api.DeleteAvailabilityResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.DeleteAvailability401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	// user has permission (approvers only)
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageTimeSlots, nil)
	if err != nil {
		logger.Error("Failed to check permission", "error", err)
		return api.DeleteAvailability500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	if !hasPermission {
		return api.DeleteAvailability403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// fetch availability to check ownership
	availability, err := s.db.Queries().GetAvailabilityByID(ctx, request.Id)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return api.DeleteAvailability404JSONResponse{
				Code:    404,
				Message: "Availability not found",
			}, nil
		}
		logger.Error("Failed to fetch availability", "error", err)
		return api.DeleteAvailability500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	// check ownership: users can only delete their own availability unless they have view_all_data permission
	if availability.UserID != nil && *availability.UserID != user.ID {
		hasGlobalPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewAllData, nil)
		if err != nil {
			logger.Error("Failed to check global permission", "error", err)
			return api.DeleteAvailability500JSONResponse{
				Code:    500,
				Message: "An unexpected error occurred",
			}, nil
		}
		if !hasGlobalPermission {
			return api.DeleteAvailability403JSONResponse{
				Code:    403,
				Message: "You can only delete your own availability",
			}, nil
		}
	}

	// referenced by bookings table?
	inUse, err := s.db.Queries().CheckAvailabilityInUse(ctx, &request.Id)
	if err != nil {
		logger.Error("Failed to check if availability is in use", "error", err)
		return api.DeleteAvailability500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	if inUse {
		return api.DeleteAvailability409JSONResponse{
			Code:    409,
			Message: "Cannot delete availability that is referenced by active bookings",
		}, nil
	}

	// delete
	err = s.db.Queries().DeleteAvailability(ctx, request.Id)
	if err != nil {
		logger.Error("Failed to delete availability", "error", err)
		return api.DeleteAvailability500JSONResponse{
			Code:    500,
			Message: "An unexpected error occurred",
		}, nil
	}

	return api.DeleteAvailability204Response{}, nil
}
