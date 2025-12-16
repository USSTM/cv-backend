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
		return api.AddToCart401JSONResponse{Code: 401, Message: "Unauthorized"}, nil
	}

	// Check permission
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageCart, &request.Body.GroupId)
	if err != nil {
		return api.AddToCart500JSONResponse{Code: 500, Message: "Internal server error"}, nil
	}
	if !hasPermission {
		return api.AddToCart403JSONResponse{Code: 403, Message: "Insufficient permissions"}, nil
	}

	// Validate item exists and has stock
	item, err := s.db.Queries().GetItemByID(ctx, request.Body.ItemId)
	if err == pgx.ErrNoRows {
		return api.AddToCart404JSONResponse{Code: 404, Message: "Item not found"}, nil
	}
	if err != nil {
		return api.AddToCart500JSONResponse{Code: 500, Message: "Internal server error"}, nil
	}

	// Check if quantity is valid
	if request.Body.Quantity <= 0 {
		return api.AddToCart400JSONResponse{Code: 400, Message: "Quantity must be greater than 0"}, nil
	}

	// Add to cart (upsert)
	cartItem, err := s.db.Queries().AddToCart(ctx, db.AddToCartParams{
		GroupID:  request.Body.GroupId,
		UserID:   user.ID,
		ItemID:   request.Body.ItemId,
		Quantity: int32(request.Body.Quantity),
	})
	if err != nil {
		return api.AddToCart500JSONResponse{Code: 500, Message: "Failed to add to cart"}, nil
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
		return api.RemoveFromCart401JSONResponse{Code: 401, Message: "Unauthorized"}, nil
	}

	// Check permission
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageCart, &request.GroupId)
	if err != nil {
		return api.RemoveFromCart500JSONResponse{Code: 500, Message: "Internal server error"}, nil
	}
	if !hasPermission {
		return api.RemoveFromCart403JSONResponse{Code: 403, Message: "Insufficient permissions"}, nil
	}

	err = s.db.Queries().RemoveFromCart(ctx, db.RemoveFromCartParams{
		GroupID: request.GroupId,
		UserID:  user.ID,
		ItemID:  request.ItemId,
	})
	if err != nil {
		return api.RemoveFromCart500JSONResponse{Code: 500, Message: "Failed to remove from cart"}, nil
	}

	return api.RemoveFromCart204Response{}, nil
}

func (s Server) GetCart(ctx context.Context, request api.GetCartRequestObject) (api.GetCartResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetCart401JSONResponse{Code: 401, Message: "Unauthorized"}, nil
	}

	// Check permission
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageCart, &request.GroupId)
	if err != nil {
		return api.GetCart500JSONResponse{Code: 500, Message: "Internal server error"}, nil
	}
	if !hasPermission {
		return api.GetCart403JSONResponse{Code: 403, Message: "Insufficient permissions"}, nil
	}

	cartItems, err := s.db.Queries().GetCartByUser(ctx, db.GetCartByUserParams{
		GroupID: request.GroupId,
		UserID:  user.ID,
	})
	if err != nil {
		return api.GetCart500JSONResponse{Code: 500, Message: "Failed to get cart"}, nil
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
		return api.UpdateCartItemQuantity401JSONResponse{Code: 401, Message: "Unauthorized"}, nil
	}

	// Check permission
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageCart, &request.GroupId)
	if err != nil {
		return api.UpdateCartItemQuantity500JSONResponse{Code: 500, Message: "Internal server error"}, nil
	}
	if !hasPermission {
		return api.UpdateCartItemQuantity403JSONResponse{Code: 403, Message: "Insufficient permissions"}, nil
	}

	// Check if quantity is valid
	if request.Body.Quantity <= 0 {
		return api.UpdateCartItemQuantity400JSONResponse{Code: 400, Message: "Quantity must be greater than 0"}, nil
	}

	cartItem, err := s.db.Queries().UpdateCartItemQuantity(ctx, db.UpdateCartItemQuantityParams{
		GroupID:  request.GroupId,
		UserID:   user.ID,
		ItemID:   request.ItemId,
		Quantity: int32(request.Body.Quantity),
	})
	if err == pgx.ErrNoRows {
		return api.UpdateCartItemQuantity404JSONResponse{Code: 404, Message: "Item not in cart"}, nil
	}
	if err != nil {
		return api.UpdateCartItemQuantity500JSONResponse{Code: 500, Message: "Failed to update quantity"}, nil
	}

	// Get item details for response
	item, err := s.db.Queries().GetItemByID(ctx, request.ItemId)
	if err != nil {
		return api.UpdateCartItemQuantity500JSONResponse{Code: 500, Message: "Internal server error"}, nil
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
		return api.ClearCart401JSONResponse{Code: 401, Message: "Unauthorized"}, nil
	}

	// Check permission
	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageCart, &request.GroupId)
	if err != nil {
		return api.ClearCart500JSONResponse{Code: 500, Message: "Internal server error"}, nil
	}
	if !hasPermission {
		return api.ClearCart403JSONResponse{Code: 403, Message: "Insufficient permissions"}, nil
	}

	err = s.db.Queries().ClearCart(ctx, db.ClearCartParams{
		GroupID: request.GroupId,
		UserID:  user.ID,
	})
	if err != nil {
		return api.ClearCart500JSONResponse{Code: 500, Message: "Failed to clear cart"}, nil
	}

	return api.ClearCart204Response{}, nil
}
