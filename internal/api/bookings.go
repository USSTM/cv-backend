package api

import (
	"context"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s Server) GetBookingByID(ctx context.Context, request api.GetBookingByIDRequestObject) (api.GetBookingByIDResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetBookingByID401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Fetch booking with joined data
	booking, err := s.db.Queries().GetBookingByID(ctx, request.BookingId)
	if err != nil {
		logger.Warn("Failed to get booking",
			"booking_id", request.BookingId,
			"error", err)
		return api.GetBookingByID404JSONResponse(NotFound("Booking").Create()), nil
	}

	// user can view own booking, or has view_all_data permission
	isOwner := booking.RequesterID != nil && *booking.RequesterID == user.ID
	hasViewAll, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewAllData, nil)
	if err != nil {
		logger.Error("Failed to check permission",
			"user_id", user.ID,
			"permission", rbac.ViewAllData,
			"error", err)
		return api.GetBookingByID500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	if !isOwner && !hasViewAll {
		return api.GetBookingByID403JSONResponse(PermissionDenied("Insufficient permissions to view this booking").Create()), nil
	}

	response := convertToBookingResponse(booking)

	return api.GetBookingByID200JSONResponse(response), nil
}

// database booking to API response
func convertToBookingResponse(booking db.GetBookingByIDRow) api.BookingResponse {
	response := api.BookingResponse{
		Id:             booking.ID,
		RequesterId:    *booking.RequesterID,
		ManagerId:      booking.ManagerID,
		ItemId:         *booking.ItemID,
		AvailabilityId: *booking.AvailabilityID,
		PickUpDate:     booking.PickUpDate.Time,
		PickUpLocation: booking.PickUpLocation,
		ReturnDate:     booking.ReturnDate.Time,
		ReturnLocation: booking.ReturnLocation,
		Status:         api.RequestStatus(booking.Status),
		CreatedAt:      booking.CreatedAt.Time,
		RequesterEmail: &booking.RequesterEmail,
		ItemName:       &booking.ItemName,
		ItemType:       (*api.ItemType)(&booking.ItemType),
	}

	response.GroupName = &booking.GroupName

	if booking.ManagerEmail.Valid {
		response.ManagerEmail = &booking.ManagerEmail.String
	}

	if booking.AvailabilityDate.Valid {
		response.AvailabilityDate = &openapi_types.Date{Time: booking.AvailabilityDate.Time}
	}

	if booking.StartTime.Valid {
		duration := time.Duration(booking.StartTime.Microseconds) * time.Microsecond
		timeStr := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC).Add(duration).Format("15:04:05")
		response.StartTime = &timeStr
	}

	if booking.EndTime.Valid {
		duration := time.Duration(booking.EndTime.Microseconds) * time.Microsecond
		timeStr := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC).Add(duration).Format("15:04:05")
		response.EndTime = &timeStr
	}

	if booking.ConfirmedAt.Valid {
		response.ConfirmedAt = &booking.ConfirmedAt.Time
	}

	if booking.ConfirmedBy != nil {
		response.ConfirmedBy = booking.ConfirmedBy
	}

	return response
}

// ListBookings row to API response format
func convertToBookingResponseFromListRow(booking db.ListBookingsRow) api.BookingResponse {
	response := api.BookingResponse{
		Id:             booking.ID,
		RequesterId:    *booking.RequesterID,
		ManagerId:      booking.ManagerID,
		ItemId:         *booking.ItemID,
		AvailabilityId: *booking.AvailabilityID,
		PickUpDate:     booking.PickUpDate.Time,
		PickUpLocation: booking.PickUpLocation,
		ReturnDate:     booking.ReturnDate.Time,
		ReturnLocation: booking.ReturnLocation,
		Status:         api.RequestStatus(booking.Status),
		CreatedAt:      booking.CreatedAt.Time,
		RequesterEmail: &booking.RequesterEmail,
		ItemName:       &booking.ItemName,
	}

	response.GroupName = &booking.GroupName

	if booking.ManagerEmail.Valid {
		response.ManagerEmail = &booking.ManagerEmail.String
	}

	if booking.AvailabilityDate.Valid {
		response.AvailabilityDate = &openapi_types.Date{Time: booking.AvailabilityDate.Time}
	}

	if booking.ConfirmedAt.Valid {
		response.ConfirmedAt = &booking.ConfirmedAt.Time
	}

	if booking.ConfirmedBy != nil {
		response.ConfirmedBy = booking.ConfirmedBy
	}

	return response
}

// ListBookingsByUser row to API response
func convertToBookingResponseFromUserRow(booking db.ListBookingsByUserRow) api.BookingResponse {
	response := api.BookingResponse{
		Id:             booking.ID,
		RequesterId:    *booking.RequesterID,
		ManagerId:      booking.ManagerID,
		ItemId:         *booking.ItemID,
		AvailabilityId: *booking.AvailabilityID,
		PickUpDate:     booking.PickUpDate.Time,
		PickUpLocation: booking.PickUpLocation,
		ReturnDate:     booking.ReturnDate.Time,
		ReturnLocation: booking.ReturnLocation,
		Status:         api.RequestStatus(booking.Status),
		CreatedAt:      booking.CreatedAt.Time,
		ItemName:       &booking.ItemName,
	}

	if booking.ManagerEmail.Valid {
		response.ManagerEmail = &booking.ManagerEmail.String
	}

	if booking.AvailabilityDate.Valid {
		response.AvailabilityDate = &openapi_types.Date{Time: booking.AvailabilityDate.Time}
	}

	if booking.StartTime.Valid {
		duration := time.Duration(booking.StartTime.Microseconds) * time.Microsecond
		timeStr := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC).Add(duration).Format("15:04:05")
		response.StartTime = &timeStr
	}

	if booking.EndTime.Valid {
		duration := time.Duration(booking.EndTime.Microseconds) * time.Microsecond
		timeStr := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC).Add(duration).Format("15:04:05")
		response.EndTime = &timeStr
	}

	if booking.ConfirmedAt.Valid {
		response.ConfirmedAt = &booking.ConfirmedAt.Time
	}

	if booking.ConfirmedBy != nil {
		response.ConfirmedBy = booking.ConfirmedBy
	}

	return response
}

// ListPendingConfirmation row to API response
func convertToBookingResponseFromPendingRow(booking db.ListPendingConfirmationRow) api.BookingResponse {
	response := api.BookingResponse{
		Id:             booking.ID,
		RequesterId:    *booking.RequesterID,
		ManagerId:      booking.ManagerID,
		ItemId:         *booking.ItemID,
		AvailabilityId: *booking.AvailabilityID,
		PickUpDate:     booking.PickUpDate.Time,
		PickUpLocation: booking.PickUpLocation,
		ReturnDate:     booking.ReturnDate.Time,
		ReturnLocation: booking.ReturnLocation,
		Status:         api.RequestStatus(booking.Status),
		CreatedAt:      booking.CreatedAt.Time,
		RequesterEmail: &booking.RequesterEmail,
		ItemName:       &booking.ItemName,
	}

	response.GroupName = &booking.GroupName

	if booking.AvailabilityDate.Valid {
		response.AvailabilityDate = &openapi_types.Date{Time: booking.AvailabilityDate.Time}
	}

	if booking.StartTime.Valid {
		duration := time.Duration(booking.StartTime.Microseconds) * time.Microsecond
		timeStr := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC).Add(duration).Format("15:04:05")
		response.StartTime = &timeStr
	}

	if booking.ConfirmedAt.Valid {
		response.ConfirmedAt = &booking.ConfirmedAt.Time
	}

	if booking.ConfirmedBy != nil {
		response.ConfirmedBy = booking.ConfirmedBy
	}

	return response
}

func (s Server) ListBookings(ctx context.Context, request api.ListBookingsRequestObject) (api.ListBookingsResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.ListBookings401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Check permissions to determine what user can view
	hasViewAll, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewAllData, nil)
	if err != nil {
		logger.Error("Failed to check view_all_data permission",
			"user_id", user.ID,
			"permission", rbac.ViewAllData,
			"error", err)
		return api.ListBookings500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	// optional API status to nullable DB status
	var status db.NullRequestStatus
	if request.Params.Status != nil {
		status = db.NullRequestStatus{
			RequestStatus: db.RequestStatus(string(*request.Params.Status)),
			Valid:         true,
		}
	}

	var fromDate, toDate pgtype.Date
	if request.Params.FromDate != nil {
		fromDate = pgtype.Date{Time: request.Params.FromDate.Time, Valid: true}
	}
	if request.Params.ToDate != nil {
		toDate = pgtype.Date{Time: request.Params.ToDate.Time, Valid: true}
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)
	var total int64

	// view_all_data, show all bookings
	if hasViewAll {
		bookings, err := s.db.Queries().ListBookings(ctx, db.ListBookingsParams{
			Status:   status,
			GroupID:  request.Params.GroupId,
			FromDate: fromDate,
			ToDate:   toDate,
			Limit:    limit,
			Offset:   offset,
		})
		if err != nil {
			logger.Error("Failed to list bookings",
				"status", status,
				"error", err)
			return api.ListBookings500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
		}

		total, err = s.db.Queries().CountBookings(ctx, db.CountBookingsParams{
			Status:   status,
			GroupID:  request.Params.GroupId,
			FromDate: fromDate,
			ToDate:   toDate,
		})
		if err != nil {
			logger.Error("Failed to count bookings", "error", err)
			return api.ListBookings500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
		}

		response := make([]api.BookingResponse, 0, len(bookings))
		for _, booking := range bookings {
			response = append(response, convertToBookingResponseFromListRow(booking))
		}
		return api.ListBookings200JSONResponse{
			Data: response,
			Meta: buildPaginationMeta(total, limit, offset),
		}, nil
	}

	// only show user's own bookings
	bookings, err := s.db.Queries().ListBookingsByUser(ctx, db.ListBookingsByUserParams{
		RequesterID: &user.ID,
		Status:      status,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		logger.Error("Failed to list bookings for user",
			"user_id", user.ID,
			"status", status,
			"error", err)
		return api.ListBookings500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	total, err = s.db.Queries().CountBookingsByUser(ctx, db.CountBookingsByUserParams{
		RequesterID: &user.ID,
		Status:      status,
	})
	if err != nil {
		logger.Error("Failed to count bookings for user", "error", err)
		return api.ListBookings500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	response := make([]api.BookingResponse, 0, len(bookings))
	for _, booking := range bookings {
		response = append(response, convertToBookingResponseFromUserRow(booking))
	}
	return api.ListBookings200JSONResponse{
		Data: response,
		Meta: buildPaginationMeta(total, limit, offset),
	}, nil
}

func (s Server) GetMyBookings(ctx context.Context, request api.GetMyBookingsRequestObject) (api.GetMyBookingsResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetMyBookings401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// optional API status to nullable DB status
	var status db.NullRequestStatus
	if request.Params.Status != nil {
		status = db.NullRequestStatus{
			RequestStatus: db.RequestStatus(string(*request.Params.Status)),
			Valid:         true,
		}
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)

	// Fetch user bookings
	bookings, err := s.db.Queries().ListBookingsByUser(ctx, db.ListBookingsByUserParams{
		RequesterID: &user.ID,
		Status:      status,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		logger.Error("Failed to list bookings for user",
			"user_id", user.ID,
			"status", status,
			"error", err)
		return api.GetMyBookings500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	total, err := s.db.Queries().CountBookingsByUser(ctx, db.CountBookingsByUserParams{
		RequesterID: &user.ID,
		Status:      status,
	})
	if err != nil {
		logger.Error("Failed to count bookings for user", "error", err)
		return api.GetMyBookings500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	response := make([]api.BookingResponse, 0, len(bookings))
	for _, booking := range bookings {
		response = append(response, convertToBookingResponseFromUserRow(booking))
	}

	return api.GetMyBookings200JSONResponse{
		Data: response,
		Meta: buildPaginationMeta(total, limit, offset),
	}, nil
}

// Permission: manage_all_bookings or manage_group_bookings
func (s Server) ListPendingConfirmation(ctx context.Context, request api.ListPendingConfirmationRequestObject) (api.ListPendingConfirmationResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.ListPendingConfirmation401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// need manage_all_bookings or manage_group_bookings
	hasManageAll, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageAllBookings, nil)
	if err != nil {
		logger.Error("Failed to check manage_all_bookings permission",
			"user_id", user.ID,
			"permission", rbac.ManageAllBookings,
			"error", err)
		return api.ListPendingConfirmation500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	var groupID *uuid.UUID

	// early return if no perms
	if !hasManageAll {
		// Check group-scoped permission
		if request.Params.GroupId != nil {
			// manage_group_bookings for request group id?
			hasManageGroup, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageGroupBookings, request.Params.GroupId)
			if err != nil {
				logger.Error("Failed to check manage_group_bookings permission",
					"user_id", user.ID,
					"permission", rbac.ManageGroupBookings,
					"group_id", request.Params.GroupId,
					"error", err)
				return api.ListPendingConfirmation500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
			}

			if !hasManageGroup {
				return api.ListPendingConfirmation403JSONResponse(PermissionDenied("Insufficient permissions to view pending confirmations for this group").Create()), nil
			}

			groupID = request.Params.GroupId
		} else {
			return api.ListPendingConfirmation400JSONResponse(ValidationErr("group_id parameter is required for group administrators", nil).Create()), nil
		}
	} else {
		// has manage_all, group doesn't matter but can be specified
		groupID = request.Params.GroupId
	}

	// Fetch pending confirmation bookings
	bookings, err := s.db.Queries().ListPendingConfirmation(ctx, groupID)
	if err != nil {
		logger.Error("Failed to list pending confirmation bookings",
			"group_id", groupID,
			"error", err)
		return api.ListPendingConfirmation500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	response := make([]api.BookingResponse, 0, len(bookings))
	for _, booking := range bookings {
		response = append(response, convertToBookingResponseFromPendingRow(booking))
	}

	return api.ListPendingConfirmation200JSONResponse(response), nil
}

// Validates: requester ownership, pending status, 48h window, before pickup
func (s Server) ConfirmBooking(ctx context.Context, request api.ConfirmBookingRequestObject) (api.ConfirmBookingResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.ConfirmBooking401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Fetch booking to validate
	booking, err := s.db.Queries().GetBookingByID(ctx, request.BookingId)
	if err != nil {
		logger.Warn("Failed to get booking for confirmation",
			"booking_id", request.BookingId,
			"error", err)
		return api.ConfirmBooking404JSONResponse(NotFound("Booking").Create()), nil
	}

	// Validate ownership
	if booking.RequesterID == nil || *booking.RequesterID != user.ID {
		return api.ConfirmBooking403JSONResponse(PermissionDenied("Only the requester can confirm this booking").Create()), nil
	}

	// Validate status
	if booking.Status != db.RequestStatusPendingConfirmation {
		return api.ConfirmBooking400JSONResponse(ValidationErr("Booking is not in pending_confirmation status", nil).Create()), nil
	}

	// Validate within 48h
	fortyEightHoursAgo := time.Now().Add(-48 * time.Hour)
	if booking.CreatedAt.Time.Before(fortyEightHoursAgo) {
		return api.ConfirmBooking400JSONResponse(ValidationErr("Confirmation window expired (must confirm within 48 hours)", nil).Create()), nil
	}

	// Validate before pickup
	if time.Now().After(booking.PickUpDate.Time) {
		return api.ConfirmBooking400JSONResponse(ValidationErr("Cannot confirm booking after pickup date has passed", nil).Create()), nil
	}

	confirmedBooking, err := s.db.Queries().ConfirmBooking(ctx, db.ConfirmBookingParams{
		ID:          request.BookingId,
		ConfirmedBy: &user.ID,
	})
	if err != nil {
		logger.Error("Failed to confirm booking",
			"booking_id", request.BookingId,
			"user_id", user.ID,
			"error", err)
		return api.ConfirmBooking500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	// complete response
	updatedBooking, err := s.db.Queries().GetBookingByID(ctx, confirmedBooking.ID)
	if err != nil {
		logger.Error("Failed to fetch confirmed booking",
			"booking_id", confirmedBooking.ID,
			"error", err)
		return api.ConfirmBooking500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	response := convertToBookingResponse(updatedBooking)
	return api.ConfirmBooking200JSONResponse(response), nil
}

// Requesters can cancel before pickup, managers/admins can cancel anytime
func (s Server) CancelBooking(ctx context.Context, request api.CancelBookingRequestObject) (api.CancelBookingResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.CancelBooking401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Fetch booking
	booking, err := s.db.Queries().GetBookingByID(ctx, request.BookingId)
	if err != nil {
		logger.Warn("Failed to get booking for cancellation",
			"booking_id", request.BookingId,
			"error", err)
		return api.CancelBooking404JSONResponse(NotFound("Booking").Create()), nil
	}

	// Check permissions
	isRequester := booking.RequesterID != nil && *booking.RequesterID == user.ID

	hasManageAll, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageAllBookings, nil)
	if err != nil {
		logger.Error("Failed to check manage_all_bookings permission",
			"user_id", user.ID,
			"permission", rbac.ManageAllBookings,
			"error", err)
		return api.CancelBooking500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	canCancel := false

	if isRequester {
		// Requester can cancel before pickup
		if time.Now().Before(booking.PickUpDate.Time) {
			canCancel = true
		}
	}

	if hasManageAll {
		// Managers/admins can always cancel
		canCancel = true
	}

	if !canCancel {
		return api.CancelBooking403JSONResponse(PermissionDenied("Insufficient permissions to cancel this booking").Create()), nil
	}

	// Cancel the booking
	_, err = s.db.Queries().CancelBooking(ctx, request.BookingId)
	if err != nil {
		logger.Error("Failed to cancel booking",
			"booking_id", request.BookingId,
			"user_id", user.ID,
			"error", err)
		return api.CancelBooking500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	// complete response
	updatedBooking, err := s.db.Queries().GetBookingByID(ctx, request.BookingId)
	if err != nil {
		logger.Error("Failed to fetch cancelled booking",
			"booking_id", request.BookingId,
			"error", err)
		return api.CancelBooking500JSONResponse(InternalError("An unexpected error occurred").Create()), nil
	}

	response := convertToBookingResponse(updatedBooking)
	return api.CancelBooking200JSONResponse(response), nil
}
