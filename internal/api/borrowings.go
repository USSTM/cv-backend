package api

import (
	"context"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Type conversion helpers

func toAPIRequestStatus(s db.NullRequestStatus) api.RequestStatus {
	return api.RequestStatus(string(s.RequestStatus))
}

func toDBRequestStatus(s api.RequestStatus) db.NullRequestStatus {
	return db.NullRequestStatus{
		RequestStatus: db.RequestStatus(string(s)),
		Valid:         true,
	}
}

func (s Server) BorrowItem(ctx context.Context, request api.BorrowItemRequestObject) (api.BorrowItemResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.BorrowItem401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	// Check permission with group scope (validates both permission and group membership)
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "request_items", &request.Body.GroupId)
	if err != nil {
		return api.BorrowItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.BorrowItem403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// transaction
	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return api.BorrowItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	defer tx.Rollback(ctx) // Auto-rollback if not committed

	qtx := s.db.Queries().WithTx(tx)

	// Lock and get item
	item, err := qtx.GetItemByIDForUpdate(ctx, request.Body.ItemId)
	if err != nil {
		return api.BorrowItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	// Reject LOW
	if item.Type == db.ItemTypeLow {
		return api.BorrowItem400JSONResponse{
			Code:    400,
			Message: "Low-value items cannot be borrowed directly. Please add to cart and checkout.",
		}, nil
	}

	// Check availability
	if item.Stock < int32(request.Body.Quantity) {
		return api.BorrowItem400JSONResponse{
			Code:    400,
			Message: "Insufficient stock available",
		}, nil
	}

	// High items checks
	var approvedRequestID *uuid.UUID
	if item.Type == db.ItemTypeHigh {
		// is currently borrowed?
		borrowable, err := qtx.CheckBorrowingItemStatus(ctx, &request.Body.ItemId)
		if err != nil {
			return api.BorrowItem500JSONResponse{
				Code:    500,
				Message: "Internal server error",
			}, nil
		}
		if !borrowable {
			return api.BorrowItem400JSONResponse{
				Code:    400,
				Message: "High-value item is currently borrowed",
			}, nil
		}
		approvedRequest, err := qtx.GetApprovedRequestForUserAndItem(ctx, db.GetApprovedRequestForUserAndItemParams{
			UserID: &user.ID,
			ItemID: &item.ID,
		})

		if err == pgx.ErrNoRows {
			return api.BorrowItem403JSONResponse{
				Code:    403,
				Message: "High-value items require an approved request. Please submit a request first.",
			}, nil
		}
		if err != nil {
			return api.BorrowItem500JSONResponse{
				Code:    500,
				Message: "Internal server error",
			}, nil
		}

		// Verify request quantity matches borrow quantity
		if approvedRequest.Quantity != int32(request.Body.Quantity) {
			return api.BorrowItem400JSONResponse{
				Code:    400,
				Message: "Borrow quantity must match approved request quantity",
			}, nil
		}

		approvedRequestID = &approvedRequest.ID
	}

	params := db.BorrowItemParams{
		UserID:             &user.ID,
		GroupID:            &request.Body.GroupId,
		ID:                 request.Body.ItemId,
		Quantity:           int32(request.Body.Quantity),
		DueDate:            pgtype.Timestamp{Time: request.Body.DueDate, Valid: true},
		BeforeCondition:    db.Condition(request.Body.BeforeCondition),
		BeforeConditionUrl: request.Body.BeforeConditionUrl,
	}

	// Create borrowing
	resp, err := qtx.BorrowItem(ctx, params)
	if err != nil {
		return api.BorrowItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	// Decrement stock
	err = qtx.DecrementItemStock(ctx, db.DecrementItemStockParams{
		ID:    item.ID,
		Stock: int32(request.Body.Quantity),
	})
	if err != nil {
		return api.BorrowItem500JSONResponse{
			Code:    500,
			Message: "Failed to update stock",
		}, nil
	}

	// If high, mark request as fulfilled
	if item.Type == db.ItemTypeHigh && approvedRequestID != nil {
		err = qtx.MarkRequestAsFulfilled(ctx, *approvedRequestID)
		if err != nil {
			return api.BorrowItem500JSONResponse{
				Code:    500,
				Message: "Failed to mark request as fulfilled",
			}, nil
		}
	}

	// end transaction
	if err := tx.Commit(ctx); err != nil {
		return api.BorrowItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	return api.BorrowItem201JSONResponse{
		Id:                 resp.ID,
		ItemId:             *resp.ItemID,
		UserId:             *resp.UserID,
		GroupId:            resp.GroupID,
		Quantity:           int(resp.Quantity),
		DueDate:            resp.DueDate.Time,
		BorrowedAt:         resp.BorrowedAt.Time,
		ReturnedAt:         nil, // set when item is returned
		BeforeCondition:    string(resp.BeforeCondition),
		BeforeConditionUrl: resp.BeforeConditionUrl,
		AfterCondition:     nil,
		AfterConditionUrl:  nil,
	}, nil
}

func (s Server) ReturnItem(ctx context.Context, request api.ReturnItemRequestObject) (api.ReturnItemResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.ReturnItem401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_own_data", nil)
	if err != nil {
		return api.ReturnItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.ReturnItem403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// transaction
	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return api.ReturnItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	defer tx.Rollback(ctx) // rollback if not committed

	qtx := s.db.Queries().WithTx(tx)

	// Get active borrowing and verify ownership (locks the row)
	_, err = qtx.GetActiveBorrowingByItemAndUser(ctx, db.GetActiveBorrowingByItemAndUserParams{
		ItemID: &request.ItemId,
		UserID: &user.ID,
	})
	if err == pgx.ErrNoRows {
		return api.ReturnItem403JSONResponse{
			Code:    403,
			Message: "Item is not actively borrowed by you, or does not exist",
		}, nil
	}
	if err != nil {
		return api.ReturnItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	// Update with return information
	params := db.ReturnItemParams{
		ItemID:            &request.ItemId,
		AfterCondition:    db.NullCondition{Condition: db.Condition(request.Body.AfterCondition), Valid: request.Body.AfterCondition != ""},
		AfterConditionUrl: pgtype.Text{String: *request.Body.AfterConditionUrl, Valid: request.Body.AfterConditionUrl != nil},
	}

	resp, err := qtx.ReturnItem(ctx, params)
	if err != nil {
		return api.ReturnItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	// Increment stock
	err = qtx.IncrementItemStock(ctx, db.IncrementItemStockParams{
		ID:    *resp.ItemID,
		Stock: resp.Quantity,
	})
	if err != nil {
		return api.ReturnItem500JSONResponse{
			Code:    500,
			Message: "Failed to update stock",
		}, nil
	}

	// end transaction
	if err := tx.Commit(ctx); err != nil {
		return api.ReturnItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	var afterCondition *string
	if resp.AfterCondition.Valid {
		conditionStr := string(resp.AfterCondition.Condition)
		afterCondition = &conditionStr
	}

	var afterConditionUrl *string
	if resp.AfterConditionUrl.Valid {
		afterConditionUrl = &resp.AfterConditionUrl.String
	}

	return api.ReturnItem200JSONResponse{
		Id:                 resp.ID,
		ItemId:             *resp.ItemID,
		UserId:             *resp.UserID,
		GroupId:            resp.GroupID,
		Quantity:           int(resp.Quantity),
		DueDate:            resp.DueDate.Time,
		BorrowedAt:         resp.BorrowedAt.Time,
		ReturnedAt:         &resp.ReturnedAt.Time,
		BeforeCondition:    string(resp.BeforeCondition),
		BeforeConditionUrl: resp.BeforeConditionUrl,
		AfterCondition:     afterCondition,
		AfterConditionUrl:  afterConditionUrl,
	}, nil
}

func (s Server) CheckBorrowingItemStatus(ctx context.Context, request api.CheckBorrowingItemStatusRequestObject) (api.CheckBorrowingItemStatusResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.CheckBorrowingItemStatus401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "request_items", nil)
	if err != nil {
		return api.CheckBorrowingItemStatus500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.CheckBorrowingItemStatus403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	borrowable, err := s.db.Queries().CheckBorrowingItemStatus(ctx, &request.ItemId)
	if err != nil {
		return api.CheckBorrowingItemStatus500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	return api.CheckBorrowingItemStatus200JSONResponse{
		IsBorrowed: &borrowable,
	}, nil
}

func (s Server) GetBorrowedItemHistoryByUserId(ctx context.Context, request api.GetBorrowedItemHistoryByUserIdRequestObject) (api.GetBorrowedItemHistoryByUserIdResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetBorrowedItemHistoryByUserId401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_own_data", nil)
	if err != nil {
		return api.GetBorrowedItemHistoryByUserId500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetBorrowedItemHistoryByUserId403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// users can only view their own borrowed items
	if user.ID != request.UserId {
		return api.GetBorrowedItemHistoryByUserId403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions to view other users' borrowed items",
		}, nil
	}

	items, err := s.db.Queries().GetBorrowedItemHistoryByUserId(ctx, &request.UserId)
	if err != nil {
		return api.GetBorrowedItemHistoryByUserId500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	borrowedItemsByUserResponse, err := createBorrowedItemResponse(items, false)
	if err != nil {
		return api.GetBorrowedItemHistoryByUserId500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	return api.GetBorrowedItemHistoryByUserId200JSONResponse(borrowedItemsByUserResponse), nil
}

func (s Server) GetActiveBorrowedItemsByUserId(ctx context.Context, request api.GetActiveBorrowedItemsByUserIdRequestObject) (api.GetActiveBorrowedItemsByUserIdResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetActiveBorrowedItemsByUserId401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_own_data", nil)
	if err != nil {
		return api.GetActiveBorrowedItemsByUserId500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetActiveBorrowedItemsByUserId403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// users can only view their own borrowed items
	if user.ID != request.UserId {
		return api.GetActiveBorrowedItemsByUserId403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions to view other users' borrowed items",
		}, nil
	}

	items, err := s.db.Queries().GetActiveBorrowedItemsByUserId(ctx, &request.UserId)
	if err != nil {
		return api.GetActiveBorrowedItemsByUserId500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	activeBorrowedItemsByUserResponse, err := createBorrowedItemResponse(items, true)
	if err != nil {
		return api.GetActiveBorrowedItemsByUserId500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	return api.GetActiveBorrowedItemsByUserId200JSONResponse(activeBorrowedItemsByUserResponse), nil
}

func (s Server) GetReturnedItemsByUserId(ctx context.Context, request api.GetReturnedItemsByUserIdRequestObject) (api.GetReturnedItemsByUserIdResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetReturnedItemsByUserId401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_own_data", nil)
	if err != nil {
		return api.GetReturnedItemsByUserId500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetReturnedItemsByUserId403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// users can only view their own borrowed items
	if user.ID != request.UserId {
		return api.GetReturnedItemsByUserId403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions to view other users' borrowed items",
		}, nil
	}

	items, err := s.db.Queries().GetReturnedItemsByUserId(ctx, &request.UserId)
	if err != nil {
		return api.GetReturnedItemsByUserId500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	returnedItemsByUserResponse, err := createBorrowedItemResponse(items, false)
	if err != nil {
		return api.GetReturnedItemsByUserId500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	return api.GetReturnedItemsByUserId200JSONResponse(returnedItemsByUserResponse), nil
}

func (s Server) GetAllActiveBorrowedItems(ctx context.Context, request api.GetAllActiveBorrowedItemsRequestObject) (api.GetAllActiveBorrowedItemsResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetAllActiveBorrowedItems401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_all_data", nil)
	if err != nil {
		return api.GetAllActiveBorrowedItems500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetAllActiveBorrowedItems403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	items, err := s.db.Queries().GetAllActiveBorrowedItems(ctx)
	if err != nil {
		return api.GetAllActiveBorrowedItems500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	activeBorrowedItemsResponse, err := createBorrowedItemResponse(items, true)
	if err != nil {
		return api.GetAllActiveBorrowedItems500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	return api.GetAllActiveBorrowedItems200JSONResponse(activeBorrowedItemsResponse), nil
}

func (s Server) GetAllReturnedItems(ctx context.Context, request api.GetAllReturnedItemsRequestObject) (api.GetAllReturnedItemsResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetAllReturnedItems401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_all_data", nil)
	if err != nil {
		return api.GetAllReturnedItems500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetAllReturnedItems403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	items, err := s.db.Queries().GetAllReturnedItems(ctx)
	if err != nil {
		return api.GetAllReturnedItems500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	returnedItemsResponse, err := createBorrowedItemResponse(items, false)
	if err != nil {
		return api.GetAllReturnedItems500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	return api.GetAllReturnedItems200JSONResponse(returnedItemsResponse), nil
}

func (s Server) GetActiveBorrowedItemsToBeReturnedByDate(ctx context.Context, request api.GetActiveBorrowedItemsToBeReturnedByDateRequestObject) (api.GetActiveBorrowedItemsToBeReturnedByDateResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetActiveBorrowedItemsToBeReturnedByDate401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_all_data", nil)
	if err != nil {
		return api.GetActiveBorrowedItemsToBeReturnedByDate500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetActiveBorrowedItemsToBeReturnedByDate403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	items, err := s.db.Queries().GetActiveBorrowedItemsToBeReturnedByDate(ctx, pgtype.Timestamp{Time: request.DueDate.Time, Valid: true})
	if err != nil {
		return api.GetActiveBorrowedItemsToBeReturnedByDate500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	borrowedItemsToBeReturnedByDateResponse, err := createBorrowedItemResponse(items, true)
	if err != nil {
		return api.GetActiveBorrowedItemsToBeReturnedByDate500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	return api.GetActiveBorrowedItemsToBeReturnedByDate200JSONResponse(borrowedItemsToBeReturnedByDateResponse), nil
}

func createBorrowedItemResponse(items []db.Borrowing, active bool) ([]api.BorrowingResponse, error) {
	var responseItems []api.BorrowingResponse

	for _, item := range items {
		var afterCondition *string
		if item.AfterCondition.Valid {
			conditionStr := string(item.AfterCondition.Condition)
			afterCondition = &conditionStr
		}

		var afterConditionUrl *string
		if item.AfterConditionUrl.Valid {
			afterConditionUrl = &item.AfterConditionUrl.String
		}

		var returnedAt *time.Time
		if active {
			returnedAt = nil
		} else {
			if item.ReturnedAt.Valid {
				returnedAt = &item.ReturnedAt.Time
			}
		}

		responseItem := api.BorrowingResponse{
			Id:                 item.ID,
			ItemId:             *item.ItemID,
			UserId:             *item.UserID,
			GroupId:            item.GroupID,
			Quantity:           int(item.Quantity),
			DueDate:            item.DueDate.Time,
			BorrowedAt:         item.BorrowedAt.Time,
			ReturnedAt:         returnedAt,
			BeforeCondition:    string(item.BeforeCondition),
			BeforeConditionUrl: item.BeforeConditionUrl,
			AfterCondition:     afterCondition,
			AfterConditionUrl:  afterConditionUrl,
		}

		responseItems = append(responseItems, responseItem)
	}

	// Return empty array instead of error when no items found
	if len(responseItems) == 0 {
		return []api.BorrowingResponse{}, nil
	}

	return responseItems, nil
}

func (s Server) RequestItem(ctx context.Context, request api.RequestItemRequestObject) (api.RequestItemResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.RequestItem401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	// Check permission with group
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "request_items", &request.Body.GroupId)
	if err != nil {
		return api.RequestItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.RequestItem403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// Validate item is high
	item, err := s.db.Queries().GetItemByID(ctx, request.Body.ItemId)
	if err == pgx.ErrNoRows {
		return api.RequestItem404JSONResponse{
			Code:    404,
			Message: "Item not found",
		}, nil
	}
	if err != nil {
		return api.RequestItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	if item.Type != db.ItemTypeHigh {
		return api.RequestItem400JSONResponse{
			Code:    400,
			Message: "Only high-value items require approval requests. Low/medium items can be borrowed directly.",
		}, nil
	}

	params := db.RequestItemParams{
		UserID:   &user.ID,
		GroupID:  &request.Body.GroupId,
		ID:       request.Body.ItemId,
		Quantity: int32(request.Body.Quantity),
	}

	resp, err := s.db.Queries().RequestItem(ctx, params)
	if err != nil {
		return api.RequestItem500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	var reviewedAt *time.Time
	if resp.ReviewedAt.Valid {
		reviewedAt = &resp.ReviewedAt.Time
	}

	return api.RequestItem201JSONResponse{
		Id:         resp.ID,
		UserId:     *resp.UserID,
		GroupId:    *resp.GroupID,
		ItemId:     *resp.ItemID,
		Quantity:   int(resp.Quantity),
		Status:     toAPIRequestStatus(resp.Status),
		ReviewedBy: resp.ReviewedBy,
		ReviewedAt: reviewedAt,
	}, nil
}

func (s Server) ReviewRequest(ctx context.Context, request api.ReviewRequestRequestObject) (api.ReviewRequestResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.ReviewRequest401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "approve_all_requests", nil)
	if err != nil {
		return api.ReviewRequest500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.ReviewRequest403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// transaction
	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return api.ReviewRequest500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	defer tx.Rollback(ctx) // rollback if not committed

	qtx := s.db.Queries().WithTx(tx)

	req, err := qtx.GetRequestByIdForUpdate(ctx, request.RequestId)
	if err == pgx.ErrNoRows {
		return api.ReviewRequest400JSONResponse{
			Code:    400,
			Message: "Request not found",
		}, nil
	}
	if err != nil {
		return api.ReviewRequest500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	// check stock
	item, err := qtx.GetItemByIDForUpdate(ctx, *req.ItemID)
	if err != nil {
		return api.ReviewRequest500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	// verify stock availability (if approved)
	if request.Body.Status == api.Approved && item.Stock < req.Quantity {
		return api.ReviewRequest400JSONResponse{
			Code:    400,
			Message: "Insufficient stock to approve this request",
		}, nil
	}

	// If approving HIGH item, create booking
	var bookingID *uuid.UUID
	if request.Body.Status == api.Approved && item.Type == db.ItemTypeHigh {
		// Validate booking fields are provided
		if request.Body.AvailabilityId == nil || request.Body.PickupLocation == nil || request.Body.ReturnLocation == nil {
			return api.ReviewRequest400JSONResponse{
				Code:    400,
				Message: "Booking fields (availability_id, pickup_location, return_location) required when approving HIGH items",
			}, nil
		}

		// Fetch availability to get date and approver
		availability, err := qtx.GetAvailabilityByID(ctx, *request.Body.AvailabilityId)
		if err != nil {
			return api.ReviewRequest400JSONResponse{
				Code:    400,
				Message: "Invalid availability_id",
			}, nil
		}

		// Calculate pickup date: availability date + time slot start time
		pickupDate := availability.Date.Time
		if availability.StartTime.Valid {
			// Convert interval microseconds directly to duration (1 microsecond = 1000 nanoseconds)
			pickupDate = pickupDate.Add(time.Duration(availability.StartTime.Microseconds) * time.Microsecond)
		}

		// Calculate return date: pickup + 7 days (default borrowing period)
		returnDate := pickupDate.Add(7 * 24 * time.Hour)

		// Create booking
		newBookingID := uuid.New()
		booking, err := qtx.CreateBooking(ctx, db.CreateBookingParams{
			ID:             newBookingID,
			RequesterID:    req.UserID,
			ManagerID:      availability.UserID,
			ItemID:         req.ItemID,
			GroupID:        req.GroupID,
			AvailabilityID: request.Body.AvailabilityId,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: *request.Body.PickupLocation,
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: *request.Body.ReturnLocation,
			Status:         db.RequestStatusPendingConfirmation,
		})
		if err != nil {
			return api.ReviewRequest500JSONResponse{
				Code:    500,
				Message: "Failed to create booking",
			}, nil
		}

		bookingID = &booking.ID

		// Link request to booking
		_, err = qtx.UpdateRequestWithBooking(ctx, db.UpdateRequestWithBookingParams{
			ID:        request.RequestId,
			BookingID: bookingID,
		})
		if err != nil {
			return api.ReviewRequest500JSONResponse{
				Code:    500,
				Message: "Failed to link request to booking",
			}, nil
		}
	}

	params := db.ReviewRequestParams{
		ID:         request.RequestId,
		Status:     toDBRequestStatus(request.Body.Status),
		ReviewedBy: &user.ID,
	}

	resp, err := qtx.ReviewRequest(ctx, params)
	if err == pgx.ErrNoRows {
		return api.ReviewRequest400JSONResponse{
			Code:    400,
			Message: "Request already reviewed or invalid",
		}, nil
	}
	if err != nil {
		return api.ReviewRequest500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	// end transaction
	if err := tx.Commit(ctx); err != nil {
		return api.ReviewRequest500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	reviewedAt := resp.ReviewedAt.Time

	return api.ReviewRequest200JSONResponse{
		Id:         resp.ID,
		UserId:     *resp.UserID,
		GroupId:    *resp.GroupID,
		ItemId:     *resp.ItemID,
		Quantity:   int(resp.Quantity),
		Status:     toAPIRequestStatus(resp.Status),
		ReviewedBy: resp.ReviewedBy,
		ReviewedAt: &reviewedAt,
	}, nil
}

func (s Server) GetAllRequests(ctx context.Context, request api.GetAllRequestsRequestObject) (api.GetAllRequestsResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetAllRequests401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_all_data", nil)
	if err != nil {
		return api.GetAllRequests500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetAllRequests403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	requests, err := s.db.Queries().GetAllRequests(ctx)
	if err != nil {
		return api.GetAllRequests500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	response := createRequestItemResponse(requests)
	return api.GetAllRequests200JSONResponse(response), nil
}

func (s Server) GetPendingRequests(ctx context.Context, request api.GetPendingRequestsRequestObject) (api.GetPendingRequestsResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetPendingRequests401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "approve_all_requests", nil)
	if err != nil {
		return api.GetPendingRequests500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetPendingRequests403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	requests, err := s.db.Queries().GetPendingRequests(ctx)
	if err != nil {
		return api.GetPendingRequests500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	response := createRequestItemResponse(requests)
	return api.GetPendingRequests200JSONResponse(response), nil
}

func (s Server) GetRequestsByUserId(ctx context.Context, request api.GetRequestsByUserIdRequestObject) (api.GetRequestsByUserIdResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetRequestsByUserId401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_own_data", nil)
	if err != nil {
		return api.GetRequestsByUserId500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetRequestsByUserId403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	// Users can only view their own requests
	if user.ID != request.UserId {
		return api.GetRequestsByUserId403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions to view other users' requests",
		}, nil
	}

	requests, err := s.db.Queries().GetRequestsByUserId(ctx, &request.UserId)
	if err != nil {
		return api.GetRequestsByUserId500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	response := createRequestItemResponse(requests)
	return api.GetRequestsByUserId200JSONResponse(response), nil
}

func (s Server) GetRequestById(ctx context.Context, request api.GetRequestByIdRequestObject) (api.GetRequestByIdResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetRequestById401JSONResponse{
			Code:    401,
			Message: "Unauthorized",
		}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_own_data", nil)
	if err != nil {
		return api.GetRequestById500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasPermission {
		return api.GetRequestById403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions",
		}, nil
	}

	req, err := s.db.Queries().GetRequestById(ctx, request.RequestId)
	if err == pgx.ErrNoRows {
		return api.GetRequestById404JSONResponse{
			Code:    404,
			Message: "Request not found",
		}, nil
	}
	if err != nil {
		return api.GetRequestById500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}

	// User can only view own requests (unless they have view_all_data permission)
	hasViewAllPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_all_data", nil)
	if err != nil {
		return api.GetRequestById500JSONResponse{
			Code:    500,
			Message: "Internal server error",
		}, nil
	}
	if !hasViewAllPermission && *req.UserID != user.ID {
		return api.GetRequestById403JSONResponse{
			Code:    403,
			Message: "Insufficient permissions to view this request",
		}, nil
	}

	var reviewedAt *time.Time
	if req.ReviewedAt.Valid {
		reviewedAt = &req.ReviewedAt.Time
	}

	return api.GetRequestById200JSONResponse{
		Id:         req.ID,
		UserId:     *req.UserID,
		GroupId:    *req.GroupID,
		ItemId:     *req.ItemID,
		Quantity:   int(req.Quantity),
		Status:     api.RequestStatus(string(req.Status.RequestStatus)),
		ReviewedBy: req.ReviewedBy,
		ReviewedAt: reviewedAt,
	}, nil
}

// Helper to convert db.Request to API response
func createRequestItemResponse(requests []db.Request) []api.RequestItemResponse {
	var response []api.RequestItemResponse

	for _, req := range requests {
		var reviewedAt *time.Time
		if req.ReviewedAt.Valid {
			reviewedAt = &req.ReviewedAt.Time
		}

		response = append(response, api.RequestItemResponse{
			Id:         req.ID,
			UserId:     *req.UserID,
			GroupId:    *req.GroupID,
			ItemId:     *req.ItemID,
			Quantity:   int(req.Quantity),
			Status:     toAPIRequestStatus(req.Status),
			ReviewedBy: req.ReviewedBy,
			ReviewedAt: reviewedAt,
		})
	}

	// Return empty array instead of nil
	if len(response) == 0 {
		return []api.RequestItemResponse{}
	}

	return response
}
