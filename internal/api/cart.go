package api

import (
	"context"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/jackc/pgx/v5"
)

func (s Server) AddToCart(ctx context.Context, request api.AddToCartRequestObject) (api.AddToCartResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.AddToCart401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Check permission
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageCart, &request.Body.GroupId)
	if err != nil {
		return api.AddToCart500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.AddToCart403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	// Validate item exists and has stock
	item, err := s.db.Queries().GetItemByID(ctx, request.Body.ItemId)
	if err == pgx.ErrNoRows {
		return api.AddToCart404JSONResponse(NotFound("Item").Create()), nil
	}
	if err != nil {
		return api.AddToCart500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	// Check if quantity is valid
	if request.Body.Quantity <= 0 {
		return api.AddToCart400JSONResponse(ValidationErr("Quantity must be greater than 0", nil).Create()), nil
	}

	// Add to cart (upsert)
	cartItem, err := s.db.Queries().AddToCart(ctx, db.AddToCartParams{
		GroupID:  request.Body.GroupId,
		UserID:   user.ID,
		ItemID:   request.Body.ItemId,
		Quantity: int32(request.Body.Quantity),
	})
	if err != nil {
		return api.AddToCart500JSONResponse(InternalError("Failed to add to cart").Create()), nil
	}

	return api.AddToCart200JSONResponse{
		GroupId:   cartItem.GroupID,
		UserId:    cartItem.UserID,
		ItemId:    cartItem.ItemID,
		ItemName:  item.Name,
		ItemType:  api.CartItemResponseItemType(string(item.Type)),
		Quantity:  int(cartItem.Quantity),
		Stock:     int(item.Stock),
		CreatedAt: cartItem.CreatedAt.Time,
	}, nil
}

func (s Server) RemoveFromCart(ctx context.Context, request api.RemoveFromCartRequestObject) (api.RemoveFromCartResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.RemoveFromCart401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Check permission
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageCart, &request.GroupId)
	if err != nil {
		return api.RemoveFromCart500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.RemoveFromCart403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	err = s.db.Queries().RemoveFromCart(ctx, db.RemoveFromCartParams{
		GroupID: request.GroupId,
		UserID:  user.ID,
		ItemID:  request.ItemId,
	})
	if err != nil {
		return api.RemoveFromCart500JSONResponse(InternalError("Failed to remove from cart").Create()), nil
	}

	return api.RemoveFromCart204Response{}, nil
}

func (s Server) GetCart(ctx context.Context, request api.GetCartRequestObject) (api.GetCartResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetCart401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Check permission
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageCart, &request.GroupId)
	if err != nil {
		return api.GetCart500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetCart403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	cartItems, err := s.db.Queries().GetCartByUser(ctx, db.GetCartByUserParams{
		GroupID: request.GroupId,
		UserID:  user.ID,
	})
	if err != nil {
		return api.GetCart500JSONResponse(InternalError("Failed to get cart").Create()), nil
	}

	var response []api.CartItemResponse
	for _, item := range cartItems {
		response = append(response, api.CartItemResponse{
			GroupId:   item.GroupID,
			UserId:    item.UserID,
			ItemId:    item.ItemID,
			Quantity:  int(item.Quantity),
			ItemName:  item.Name,
			ItemType:  api.CartItemResponseItemType(string(item.Type)),
			Stock:     int(item.Stock),
			CreatedAt: item.CreatedAt.Time,
		})
	}

	// Return empty array instead of nil
	if len(response) == 0 {
		return api.GetCart200JSONResponse([]api.CartItemResponse{}), nil
	}

	return api.GetCart200JSONResponse(response), nil
}

func (s Server) UpdateCartItemQuantity(ctx context.Context, request api.UpdateCartItemQuantityRequestObject) (api.UpdateCartItemQuantityResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.UpdateCartItemQuantity401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Check permission
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageCart, &request.GroupId)
	if err != nil {
		return api.UpdateCartItemQuantity500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.UpdateCartItemQuantity403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	// Check if quantity is valid
	if request.Body.Quantity <= 0 {
		return api.UpdateCartItemQuantity400JSONResponse(ValidationErr("Quantity must be greater than 0", nil).Create()), nil
	}

	cartItem, err := s.db.Queries().UpdateCartItemQuantity(ctx, db.UpdateCartItemQuantityParams{
		GroupID:  request.GroupId,
		UserID:   user.ID,
		ItemID:   request.ItemId,
		Quantity: int32(request.Body.Quantity),
	})
	if err == pgx.ErrNoRows {
		return api.UpdateCartItemQuantity404JSONResponse(NotFound("Item not in cart").Create()), nil
	}
	if err != nil {
		return api.UpdateCartItemQuantity500JSONResponse(InternalError("Failed to update quantity").Create()), nil
	}

	// Get item details for response
	item, err := s.db.Queries().GetItemByID(ctx, request.ItemId)
	if err != nil {
		return api.UpdateCartItemQuantity500JSONResponse(InternalError("Internal server error").Create()), nil
	}

	return api.UpdateCartItemQuantity200JSONResponse{
		GroupId:  cartItem.GroupID,
		UserId:   cartItem.UserID,
		ItemId:   cartItem.ItemID,
		ItemName: item.Name,
		ItemType: api.CartItemResponseItemType(string(item.Type)),
		Quantity: int(cartItem.Quantity),
		Stock:    int(item.Stock),
	}, nil
}

func (s Server) ClearCart(ctx context.Context, request api.ClearCartRequestObject) (api.ClearCartResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.ClearCart401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Check permission
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageCart, &request.GroupId)
	if err != nil {
		return api.ClearCart500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.ClearCart403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	err = s.db.Queries().ClearCart(ctx, db.ClearCartParams{
		GroupID: request.GroupId,
		UserID:  user.ID,
	})
	if err != nil {
		return api.ClearCart500JSONResponse(InternalError("Failed to clear cart").Create()), nil
	}

	return api.ClearCart204Response{}, nil
}
