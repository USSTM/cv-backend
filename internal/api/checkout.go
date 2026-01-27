package api

import (
	"context"
	"fmt"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type CheckoutResult struct {
	LowItemsProcessed   []api.CheckoutItemResult
	MediumItemsBorrowed []api.CheckoutItemResult
	HighItemsRequested  []api.CheckoutItemResult
	Errors              []api.CheckoutError
}

func (s Server) CheckoutCart(ctx context.Context, request api.CheckoutCartRequestObject) (api.CheckoutCartResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.CheckoutCart401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Check permission
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.RequestItems, &request.Body.GroupId)
	if err != nil {
		logger.Error("Failed to check request_items permission",
			"user_id", user.ID,
			"group_id", request.Body.GroupId,
			"permission", rbac.RequestItems,
			"error", err)
		return api.CheckoutCart500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.CheckoutCart403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	// transaction
	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		logger.Error("Failed to begin checkout transaction",
			"user_id", user.ID,
			"group_id", request.Body.GroupId,
			"error", err)
		return api.CheckoutCart500JSONResponse(InternalError("Failed to start transaction").Create()), nil
	}
	defer tx.Rollback(ctx)

	qtx := s.db.Queries().WithTx(tx)

	// Get all cart items
	cartItems, err := qtx.GetCartItemsForCheckout(ctx, db.GetCartItemsForCheckoutParams{
		GroupID: request.Body.GroupId,
		UserID:  user.ID,
	})
	if err != nil {
		logger.Error("Failed to get cart items for checkout",
			"user_id", user.ID,
			"group_id", request.Body.GroupId,
			"error", err)
		return api.CheckoutCart500JSONResponse(InternalError("Failed to get cart items").Create()), nil
	}

	if len(cartItems) == 0 {
		return api.CheckoutCart400JSONResponse(ValidationErr("Cart is empty", nil).Create()), nil
	}

	result := CheckoutResult{
		LowItemsProcessed:   []api.CheckoutItemResult{},
		MediumItemsBorrowed: []api.CheckoutItemResult{},
		HighItemsRequested:  []api.CheckoutItemResult{},
		Errors:              []api.CheckoutError{},
	}

	// Process each cart item based on type
	for _, cartItem := range cartItems {
		switch cartItem.Type {
		case db.ItemTypeLow:
			err := s.processLowItem(ctx, qtx, cartItem, request.Body.GroupId, user.ID, &result)
			if err != nil {
				logger.Warn("Failed to process LOW item in checkout",
					"item_id", cartItem.ItemID,
					"item_name", cartItem.Name,
					"user_id", user.ID,
					"error", err)
				itemName := cartItem.Name
				result.Errors = append(result.Errors, api.CheckoutError{
					ItemId:   cartItem.ItemID,
					ItemName: &itemName,
					Message:  err.Error(),
				})
			}

		case db.ItemTypeMedium:
			err := s.processMediumItem(ctx, qtx, cartItem, request.Body, user.ID, &result)
			if err != nil {
				logger.Warn("Failed to process MEDIUM item in checkout",
					"item_id", cartItem.ItemID,
					"item_name", cartItem.Name,
					"user_id", user.ID,
					"error", err)
				itemName := cartItem.Name
				result.Errors = append(result.Errors, api.CheckoutError{
					ItemId:   cartItem.ItemID,
					ItemName: &itemName,
					Message:  err.Error(),
				})
			}

		case db.ItemTypeHigh:
			err := s.processHighItem(ctx, qtx, cartItem, request.Body.GroupId, user.ID, &result)
			if err != nil {
				logger.Warn("Failed to process HIGH item in checkout",
					"item_id", cartItem.ItemID,
					"item_name", cartItem.Name,
					"user_id", user.ID,
					"error", err)
				itemName := cartItem.Name
				result.Errors = append(result.Errors, api.CheckoutError{
					ItemId:   cartItem.ItemID,
					ItemName: &itemName,
					Message:  err.Error(),
				})
			}
		}
	}

	// Clear cart
	err = qtx.ClearCart(ctx, db.ClearCartParams{
		GroupID: request.Body.GroupId,
		UserID:  user.ID,
	})
	if err != nil {
		return api.CheckoutCart500JSONResponse(InternalError("Failed to clear cart").Create()), nil
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return api.CheckoutCart500JSONResponse(InternalError("Failed to commit transaction").Create()), nil
	}

	return api.CheckoutCart200JSONResponse{
		LowItemsProcessed:   result.LowItemsProcessed,
		MediumItemsBorrowed: result.MediumItemsBorrowed,
		HighItemsRequested:  result.HighItemsRequested,
		Errors:              result.Errors,
	}, nil
}

// decrement stock + record taking for audit, no borrowing
func (s Server) processLowItem(ctx context.Context, qtx *db.Queries, cartItem db.GetCartItemsForCheckoutRow,
	groupID uuid.UUID, userID uuid.UUID, result *CheckoutResult) error {

	// Validate
	if cartItem.Stock < cartItem.Quantity {
		return fmt.Errorf("insufficient stock (requested: %d, available: %d)",
			cartItem.Quantity, cartItem.Stock)
	}

	// Decrement
	err := qtx.DecrementStockForLowItem(ctx, db.DecrementStockForLowItemParams{
		ID:    cartItem.ItemID,
		Stock: cartItem.Quantity,
	})
	if err != nil {
		return fmt.Errorf("failed to decrement stock: %w", err)
	}

	// audit trail
	taking, err := qtx.RecordItemTaking(ctx, db.RecordItemTakingParams{
		UserID:   userID,
		GroupID:  groupID,
		ItemID:   cartItem.ItemID,
		Quantity: cartItem.Quantity,
	})
	if err != nil {
		return fmt.Errorf("failed to record taking: %w", err)
	}

	result.LowItemsProcessed = append(result.LowItemsProcessed, api.CheckoutItemResult{
		ItemId:   cartItem.ItemID,
		ItemName: cartItem.Name,
		Quantity: int(cartItem.Quantity),
		Status:   rbac.CheckoutStatusCompleted,
		TakingId: &taking.ID,
	})

	return nil
}

// borrowing record + decrement stock
func (s Server) processMediumItem(ctx context.Context, qtx *db.Queries, cartItem db.GetCartItemsForCheckoutRow,
	requestBody *api.CheckoutCartJSONRequestBody, userID uuid.UUID, result *CheckoutResult) error {

	// Validate
	if cartItem.Stock < cartItem.Quantity {
		return fmt.Errorf("insufficient stock (requested: %d, available: %d)",
			cartItem.Quantity, cartItem.Stock)
	}

	// Create record
	borrowing, err := qtx.BorrowItem(ctx, db.BorrowItemParams{
		UserID:             &userID,
		GroupID:            &requestBody.GroupId,
		ID:                 cartItem.ItemID,
		Quantity:           cartItem.Quantity,
		DueDate:            pgtype.Timestamp{Time: requestBody.DueDate, Valid: true},
		BeforeCondition:    db.Condition(requestBody.BeforeCondition),
		BeforeConditionUrl: requestBody.BeforeConditionUrl,
	})
	if err != nil {
		return fmt.Errorf("failed to create borrowing: %w", err)
	}

	// Decrement
	err = qtx.DecrementItemStock(ctx, db.DecrementItemStockParams{
		ID:    cartItem.ItemID,
		Stock: cartItem.Quantity,
	})
	if err != nil {
		return fmt.Errorf("failed to decrement stock: %w", err)
	}

	result.MediumItemsBorrowed = append(result.MediumItemsBorrowed, api.CheckoutItemResult{
		ItemId:      cartItem.ItemID,
		ItemName:    cartItem.Name,
		Quantity:    int(cartItem.Quantity),
		Status:      rbac.CheckoutStatusBorrowed,
		BorrowingId: &borrowing.ID,
	})

	return nil
}

// approval request
func (s Server) processHighItem(ctx context.Context, qtx *db.Queries, cartItem db.GetCartItemsForCheckoutRow,
	groupID uuid.UUID, userID uuid.UUID, result *CheckoutResult) error {

	// Create request
	request, err := qtx.RequestItem(ctx, db.RequestItemParams{
		UserID:   &userID,
		GroupID:  &groupID,
		ID:       cartItem.ItemID,
		Quantity: cartItem.Quantity,
	})
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	result.HighItemsRequested = append(result.HighItemsRequested, api.CheckoutItemResult{
		ItemId:    cartItem.ItemID,
		ItemName:  cartItem.Name,
		Quantity:  int(cartItem.Quantity),
		Status:    rbac.CheckoutStatusPendingApproval,
		RequestId: &request.ID,
	})

	return nil
}
