package api

import (
	"context"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/rbac"
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
		return api.BorrowItem401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Check permission with group scope (validates both permission and group membership)
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.RequestItems, &request.Body.GroupId)
	if err != nil {
		return api.BorrowItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.BorrowItem403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	// transaction
	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return api.BorrowItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	defer tx.Rollback(ctx) // Auto-rollback if not committed

	qtx := s.db.Queries().WithTx(tx)

	// Lock and get item
	item, err := qtx.GetItemByIDForUpdate(ctx, request.Body.ItemId)
	if err != nil {
		return api.BorrowItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	// Reject LOW
	if item.Type == db.ItemTypeLow {
		return api.BorrowItem400JSONResponse(ValidationErr("Low-value items cannot be borrowed directly. Please add to cart and checkout.", nil).Create()), nil
	}

	// Check availability
	if item.Stock < int32(request.Body.Quantity) {
		return api.BorrowItem400JSONResponse(ValidationErr("Insufficient stock available", nil).Create()), nil
	}

	// High items checks
	var approvedRequestID *uuid.UUID
	if item.Type == db.ItemTypeHigh {
		// is currently borrowed?
		borrowable, err := qtx.CheckBorrowingItemStatus(ctx, &request.Body.ItemId)
		if err != nil {
			return api.BorrowItem500JSONResponse(InternalError("Internal server error").Create()), nil
		}
		if !borrowable {
			return api.BorrowItem400JSONResponse(ValidationErr("High-value item is currently borrowed", nil).Create()), nil
		}
		approvedRequest, err := qtx.GetApprovedRequestForUserAndItem(ctx, db.GetApprovedRequestForUserAndItemParams{
			UserID: &user.ID,
			ItemID: &item.ID,
		})

		if err == pgx.ErrNoRows {
			return api.BorrowItem403JSONResponse(PermissionDenied("High-value items require an approved request. Please submit a request first.").Create()), nil
		}
		if err != nil {
			return api.BorrowItem500JSONResponse(InternalError("Internal server error").Create()), nil
		}

		// Verify request quantity matches borrow quantity
		if approvedRequest.Quantity != int32(request.Body.Quantity) {
			return api.BorrowItem400JSONResponse(ValidationErr("Borrow quantity must match approved request quantity", nil).Create()), nil
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
		return api.BorrowItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	// Decrement stock
	err = qtx.DecrementItemStock(ctx, db.DecrementItemStockParams{
		ID:    item.ID,
		Stock: int32(request.Body.Quantity),
	})
	if err != nil {
		return api.BorrowItem500JSONResponse(InternalError("Failed to update stock").Create()), nil
	}

	// If high, mark request as fulfilled
	if item.Type == db.ItemTypeHigh && approvedRequestID != nil {
		err = qtx.MarkRequestAsFulfilled(ctx, *approvedRequestID)
		if err != nil {
			return api.BorrowItem500JSONResponse(InternalError("Failed to mark request as fulfilled").Create()), nil
		}
	}

	// end transaction
	if err := tx.Commit(ctx); err != nil {
		return api.BorrowItem500JSONResponse(InternalError("Internal server error").Create()), nil
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
		return api.ReturnItem401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewOwnData, nil)
	if err != nil {
		return api.ReturnItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.ReturnItem403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	// transaction
	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return api.ReturnItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	defer tx.Rollback(ctx) // rollback if not committed

	qtx := s.db.Queries().WithTx(tx)

	// Get active borrowing and verify ownership (locks the row)
	_, err = qtx.GetActiveBorrowingByItemAndUser(ctx, db.GetActiveBorrowingByItemAndUserParams{
		ItemID: &request.ItemId,
		UserID: &user.ID,
	})
	if err == pgx.ErrNoRows {
		return api.ReturnItem403JSONResponse(PermissionDenied("Item is not actively borrowed by you, or does not exist").Create()), nil
	}
	if err != nil {
		return api.ReturnItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	// Update with return information
	params := db.ReturnItemParams{
		ItemID:            &request.ItemId,
		AfterCondition:    db.NullCondition{Condition: db.Condition(request.Body.AfterCondition), Valid: request.Body.AfterCondition != ""},
		AfterConditionUrl: pgtype.Text{String: *request.Body.AfterConditionUrl, Valid: request.Body.AfterConditionUrl != nil},
	}

	resp, err := qtx.ReturnItem(ctx, params)
	if err != nil {
		return api.ReturnItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	// Increment stock
	err = qtx.IncrementItemStock(ctx, db.IncrementItemStockParams{
		ID:    *resp.ItemID,
		Stock: resp.Quantity,
	})
	if err != nil {
		return api.ReturnItem500JSONResponse(InternalError("Failed to update stock").Create()), nil
	}

	// end transaction
	if err := tx.Commit(ctx); err != nil {
		return api.ReturnItem500JSONResponse(InternalError("Internal server error").Create()), nil
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
		return api.CheckBorrowingItemStatus401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.RequestItems, nil)
	if err != nil {
		return api.CheckBorrowingItemStatus500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.CheckBorrowingItemStatus403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	borrowable, err := s.db.Queries().CheckBorrowingItemStatus(ctx, &request.ItemId)
	if err != nil {
		return api.CheckBorrowingItemStatus500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	return api.CheckBorrowingItemStatus200JSONResponse{
		IsBorrowed: &borrowable,
	}, nil
}

func (s Server) GetBorrowedItemHistoryByUserId(ctx context.Context, request api.GetBorrowedItemHistoryByUserIdRequestObject) (api.GetBorrowedItemHistoryByUserIdResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetBorrowedItemHistoryByUserId401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewOwnData, nil)
	if err != nil {
		return api.GetBorrowedItemHistoryByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetBorrowedItemHistoryByUserId403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	// users can only view their own borrowed items
	if user.ID != request.UserId {
		return api.GetBorrowedItemHistoryByUserId403JSONResponse(PermissionDenied("Insufficient permissions to view other users' borrowed items").Create()), nil
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)

	items, err := s.db.Queries().GetBorrowedItemHistoryByUserId(ctx, db.GetBorrowedItemHistoryByUserIdParams{
		UserID: &request.UserId,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return api.GetBorrowedItemHistoryByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	total, err := s.db.Queries().CountBorrowedItemHistoryByUserId(ctx, &request.UserId)
	if err != nil {
		return api.GetBorrowedItemHistoryByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	borrowedItemsByUserResponse, err := createBorrowedItemResponse(items, false)
	if err != nil {
		return api.GetBorrowedItemHistoryByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	return api.GetBorrowedItemHistoryByUserId200JSONResponse{
		Data: borrowedItemsByUserResponse,
		Meta: buildPaginationMeta(total, limit, offset),
	}, nil
}

func (s Server) GetActiveBorrowedItemsByUserId(ctx context.Context, request api.GetActiveBorrowedItemsByUserIdRequestObject) (api.GetActiveBorrowedItemsByUserIdResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetActiveBorrowedItemsByUserId401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewOwnData, nil)
	if err != nil {
		return api.GetActiveBorrowedItemsByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetActiveBorrowedItemsByUserId403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	// users can only view their own borrowed items
	if user.ID != request.UserId {
		return api.GetActiveBorrowedItemsByUserId403JSONResponse(PermissionDenied("Insufficient permissions to view other users' borrowed items").Create()), nil
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)

	items, err := s.db.Queries().GetActiveBorrowedItemsByUserId(ctx, db.GetActiveBorrowedItemsByUserIdParams{
		UserID: &request.UserId,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return api.GetActiveBorrowedItemsByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	total, err := s.db.Queries().CountActiveBorrowedItemsByUserId(ctx, &request.UserId)
	if err != nil {
		return api.GetActiveBorrowedItemsByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	activeBorrowedItemsByUserResponse, err := createBorrowedItemResponse(items, true)
	if err != nil {
		return api.GetActiveBorrowedItemsByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	return api.GetActiveBorrowedItemsByUserId200JSONResponse{
		Data: activeBorrowedItemsByUserResponse,
		Meta: buildPaginationMeta(total, limit, offset),
	}, nil
}

func (s Server) GetReturnedItemsByUserId(ctx context.Context, request api.GetReturnedItemsByUserIdRequestObject) (api.GetReturnedItemsByUserIdResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetReturnedItemsByUserId401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewOwnData, nil)
	if err != nil {
		return api.GetReturnedItemsByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetReturnedItemsByUserId403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	// users can only view their own borrowed items
	if user.ID != request.UserId {
		return api.GetReturnedItemsByUserId403JSONResponse(PermissionDenied("Insufficient permissions to view other users' borrowed items").Create()), nil
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)

	items, err := s.db.Queries().GetReturnedItemsByUserId(ctx, db.GetReturnedItemsByUserIdParams{
		UserID: &request.UserId,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return api.GetReturnedItemsByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	total, err := s.db.Queries().CountReturnedItemsByUserId(ctx, &request.UserId)
	if err != nil {
		return api.GetReturnedItemsByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	returnedItemsByUserResponse, err := createBorrowedItemResponse(items, false)
	if err != nil {
		return api.GetReturnedItemsByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	return api.GetReturnedItemsByUserId200JSONResponse{
		Data: returnedItemsByUserResponse,
		Meta: buildPaginationMeta(total, limit, offset),
	}, nil
}

func (s Server) GetAllActiveBorrowedItems(ctx context.Context, request api.GetAllActiveBorrowedItemsRequestObject) (api.GetAllActiveBorrowedItemsResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetAllActiveBorrowedItems401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewAllData, nil)
	if err != nil {
		return api.GetAllActiveBorrowedItems500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetAllActiveBorrowedItems403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)

	items, err := s.db.Queries().GetAllActiveBorrowedItems(ctx, db.GetAllActiveBorrowedItemsParams{Limit: limit, Offset: offset})
	if err != nil {
		return api.GetAllActiveBorrowedItems500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	total, err := s.db.Queries().CountAllActiveBorrowedItems(ctx)
	if err != nil {
		return api.GetAllActiveBorrowedItems500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	activeBorrowedItemsResponse, err := createBorrowedItemResponse(items, true)
	if err != nil {
		return api.GetAllActiveBorrowedItems500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	return api.GetAllActiveBorrowedItems200JSONResponse{
		Data: activeBorrowedItemsResponse,
		Meta: buildPaginationMeta(total, limit, offset),
	}, nil
}

func (s Server) GetAllReturnedItems(ctx context.Context, request api.GetAllReturnedItemsRequestObject) (api.GetAllReturnedItemsResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetAllReturnedItems401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewAllData, nil)
	if err != nil {
		return api.GetAllReturnedItems500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetAllReturnedItems403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)

	items, err := s.db.Queries().GetAllReturnedItems(ctx, db.GetAllReturnedItemsParams{Limit: limit, Offset: offset})
	if err != nil {
		return api.GetAllReturnedItems500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	total, err := s.db.Queries().CountAllReturnedItems(ctx)
	if err != nil {
		return api.GetAllReturnedItems500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	returnedItemsResponse, err := createBorrowedItemResponse(items, false)
	if err != nil {
		return api.GetAllReturnedItems500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	return api.GetAllReturnedItems200JSONResponse{
		Data: returnedItemsResponse,
		Meta: buildPaginationMeta(total, limit, offset),
	}, nil
}

func (s Server) GetActiveBorrowedItemsToBeReturnedByDate(ctx context.Context, request api.GetActiveBorrowedItemsToBeReturnedByDateRequestObject) (api.GetActiveBorrowedItemsToBeReturnedByDateResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetActiveBorrowedItemsToBeReturnedByDate401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewAllData, nil)
	if err != nil {
		return api.GetActiveBorrowedItemsToBeReturnedByDate500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetActiveBorrowedItemsToBeReturnedByDate403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	items, err := s.db.Queries().GetActiveBorrowedItemsToBeReturnedByDate(ctx, pgtype.Timestamp{Time: request.DueDate.Time, Valid: true})
	if err != nil {
		return api.GetActiveBorrowedItemsToBeReturnedByDate500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	borrowedItemsToBeReturnedByDateResponse, err := createBorrowedItemResponse(items, true)
	if err != nil {
		return api.GetActiveBorrowedItemsToBeReturnedByDate500JSONResponse(InternalError("Internal server error").Create()), nil
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
		return api.RequestItem401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Check permission with group
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.RequestItems, &request.Body.GroupId)
	if err != nil {
		return api.RequestItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.RequestItem403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	// Validate item is high
	item, err := s.db.Queries().GetItemByID(ctx, request.Body.ItemId)
	if err == pgx.ErrNoRows {
		return api.RequestItem404JSONResponse(NotFound("Item").Create()), nil
	}
	if err != nil {
		return api.RequestItem500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	if item.Type != db.ItemTypeHigh {
		return api.RequestItem400JSONResponse(ValidationErr("Only high-value items require approval requests. Low/medium items can be borrowed directly.", nil).Create()), nil
	}

	params := db.RequestItemParams{
		UserID:   &user.ID,
		GroupID:  &request.Body.GroupId,
		ID:       request.Body.ItemId,
		Quantity: int32(request.Body.Quantity),
	}

	resp, err := s.db.Queries().RequestItem(ctx, params)
	if err != nil {
		return api.RequestItem500JSONResponse(InternalError("Internal server error").Create()), nil
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
		return api.ReviewRequest401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ApproveAllRequests, nil)
	if err != nil {
		return api.ReviewRequest500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.ReviewRequest403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	// transaction
	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return api.ReviewRequest500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	defer tx.Rollback(ctx) // rollback if not committed

	qtx := s.db.Queries().WithTx(tx)

	req, err := qtx.GetRequestByIdForUpdate(ctx, request.RequestId)
	if err == pgx.ErrNoRows {
		return api.ReviewRequest400JSONResponse(ValidationErr("Request not found", nil).Create()), nil
	}
	if err != nil {
		return api.ReviewRequest500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	// check stock
	item, err := qtx.GetItemByIDForUpdate(ctx, *req.ItemID)
	if err != nil {
		return api.ReviewRequest500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	// verify stock availability (if approved)
	if request.Body.Status == api.Approved && item.Stock < req.Quantity {
		return api.ReviewRequest400JSONResponse(ValidationErr("Insufficient stock to approve this request", nil).Create()), nil
	}

	// If approving HIGH item, create booking
	var bookingID *uuid.UUID
	if request.Body.Status == api.Approved && item.Type == db.ItemTypeHigh {
		// Validate booking fields are provided
		if request.Body.AvailabilityId == nil || request.Body.PickupLocation == nil || request.Body.ReturnLocation == nil {
			return api.ReviewRequest400JSONResponse(ValidationErr("Booking fields (availability_id, pickup_location, return_location) required when approving HIGH items", nil).Create()), nil
		}

		// Fetch availability to get date and approver
		availability, err := qtx.GetAvailabilityByID(ctx, *request.Body.AvailabilityId)
		if err != nil {
			return api.ReviewRequest400JSONResponse(ValidationErr("Invalid availability_id", nil).Create()), nil
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
			return api.ReviewRequest500JSONResponse(InternalError("Failed to create booking").Create()), nil
		}

		bookingID = &booking.ID

		// Link request to booking
		_, err = qtx.UpdateRequestWithBooking(ctx, db.UpdateRequestWithBookingParams{
			ID:        request.RequestId,
			BookingID: bookingID,
		})
		if err != nil {
			return api.ReviewRequest500JSONResponse(InternalError("Failed to link request to booking").Create()), nil
		}
	}

	params := db.ReviewRequestParams{
		ID:         request.RequestId,
		Status:     toDBRequestStatus(request.Body.Status),
		ReviewedBy: &user.ID,
	}

	resp, err := qtx.ReviewRequest(ctx, params)
	if err == pgx.ErrNoRows {
		return api.ReviewRequest400JSONResponse(ValidationErr("Request already reviewed or invalid", nil).Create()), nil
	}
	if err != nil {
		return api.ReviewRequest500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	// end transaction
	if err := tx.Commit(ctx); err != nil {
		return api.ReviewRequest500JSONResponse(InternalError("Internal server error").Create()), nil
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
		return api.GetAllRequests401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewAllData, nil)
	if err != nil {
		return api.GetAllRequests500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetAllRequests403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)

	requests, err := s.db.Queries().GetAllRequests(ctx, db.GetAllRequestsParams{Limit: limit, Offset: offset})
	if err != nil {
		return api.GetAllRequests500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	total, err := s.db.Queries().CountAllRequests(ctx)
	if err != nil {
		return api.GetAllRequests500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	response := createRequestItemResponse(requests)
	return api.GetAllRequests200JSONResponse{
		Data: response,
		Meta: buildPaginationMeta(total, limit, offset),
	}, nil
}

func (s Server) GetPendingRequests(ctx context.Context, request api.GetPendingRequestsRequestObject) (api.GetPendingRequestsResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetPendingRequests401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ApproveAllRequests, nil)
	if err != nil {
		return api.GetPendingRequests500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetPendingRequests403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	limit, offset := parsePagination(request.Params.Limit, request.Params.Offset)

	requests, err := s.db.Queries().GetPendingRequests(ctx, db.GetPendingRequestsParams{Limit: limit, Offset: offset})
	if err != nil {
		return api.GetPendingRequests500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	total, err := s.db.Queries().CountPendingRequests(ctx)
	if err != nil {
		return api.GetPendingRequests500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	response := createRequestItemResponse(requests)
	return api.GetPendingRequests200JSONResponse{
		Data: response,
		Meta: buildPaginationMeta(total, limit, offset),
	}, nil
}

func (s Server) GetRequestsByUserId(ctx context.Context, request api.GetRequestsByUserIdRequestObject) (api.GetRequestsByUserIdResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetRequestsByUserId401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewOwnData, nil)
	if err != nil {
		return api.GetRequestsByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetRequestsByUserId403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	// Users can only view their own requests
	if user.ID != request.UserId {
		return api.GetRequestsByUserId403JSONResponse(PermissionDenied("Insufficient permissions to view other users' requests").Create()), nil
	}

	requests, err := s.db.Queries().GetRequestsByUserId(ctx, &request.UserId)
	if err != nil {
		return api.GetRequestsByUserId500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	response := createRequestItemResponse(requests)
	return api.GetRequestsByUserId200JSONResponse(response), nil
}

func (s Server) GetRequestById(ctx context.Context, request api.GetRequestByIdRequestObject) (api.GetRequestByIdResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetRequestById401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewOwnData, nil)
	if err != nil {
		return api.GetRequestById500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetRequestById403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	req, err := s.db.Queries().GetRequestById(ctx, request.RequestId)
	if err == pgx.ErrNoRows {
		return api.GetRequestById404JSONResponse(NotFound("Request").Create()), nil
	}
	if err != nil {
		return api.GetRequestById500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	// User can only view own requests (unless they have rbac.ViewAllData permission)
	hasViewAllPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewAllData, nil)
	if err != nil {
		return api.GetRequestById500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasViewAllPermission && *req.UserID != user.ID {
		return api.GetRequestById403JSONResponse(PermissionDenied("Insufficient permissions to view this request").Create()), nil
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
