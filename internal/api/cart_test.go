package api

import (
	"context"
	"testing"

	"github.com/USSTM/cv-backend/internal/rbac"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCartServer(t *testing.T) (*Server, *testutil.TestDatabase, *testutil.MockAuthenticator) {
	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)

	server := NewServer(testDB, testQueue, mockJWT, mockAuth)

	return server, testDB, mockAuth
}

func TestServer_AddToCart(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration tests")
	}

	server, testDB, mockAuth := testCartServer(t)

	t.Run("successful add item to cart", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@add.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Add Cart Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Projector").
			WithType("medium").
			WithStock(10).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item.ID,
				Quantity: 3,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.AddToCart200JSONResponse{}, response)

		cartResp := response.(api.AddToCart200JSONResponse)
		assert.Equal(t, group.ID, cartResp.GroupId)
		assert.Equal(t, testUser.ID, cartResp.UserId)
		assert.Equal(t, item.ID, cartResp.ItemId)
		assert.Equal(t, 3, cartResp.Quantity)
		assert.Equal(t, 10, cartResp.Stock)
	})

	t.Run("add item to cart increments quantity if already exists", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@update.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Update Cart Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Camera").
			WithType("medium").
			WithStock(5).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		// Add item first time
		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item.ID,
				Quantity: 2,
			},
		})
		require.NoError(t, err)

		// Add same item again (should increment)
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)

		response, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item.ID,
				Quantity: 1,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.AddToCart200JSONResponse{}, response)

		cartResp := response.(api.AddToCart200JSONResponse)
		// The quantity should be 2 + 1 = 3 (upsert)
		assert.Equal(t, 3, cartResp.Quantity)
	})

	t.Run("cannot add item with zero quantity", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@zeroquantity.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Zero Quantity Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Microphone").
			WithType("medium").
			WithStock(5).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item.ID,
				Quantity: 0,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.AddToCart400JSONResponse{}, response)

		errorResp := response.(api.AddToCart400JSONResponse)
		assert.Equal(t, int32(400), errorResp.Code)
		assert.Contains(t, errorResp.Message, "greater than 0")
	})

	t.Run("cannot add non-existent item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@nonexistent.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Nonexistent Item Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   uuid.New(),
				Quantity: 1,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.AddToCart404JSONResponse{}, response)

		errorResp := response.(api.AddToCart404JSONResponse)
		assert.Equal(t, int32(404), errorResp.Code)
		assert.Contains(t, errorResp.Message, "not found")
	})

	t.Run("user cannot add item to cart for group they are not member of", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@notmember.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Restricted Cart Group").
			Create()

		item := testDB.NewItem(t).
			WithName("Laptop").
			WithType("medium").
			WithStock(5).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item.ID,
				Quantity: 1,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.AddToCart403JSONResponse{}, response)

		errorResp := response.(api.AddToCart403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient permissions")
	})

	// hypothetical, this is edge case where user role assignment gets bugged
	t.Run("user without permission cannot add to cart", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@noperm.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("No Permission Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Keyboard").
			WithType("low").
			WithStock(20).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item.ID,
				Quantity: 2,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.AddToCart403JSONResponse{}, response)

		errorResp := response.(api.AddToCart403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Equal(t, "Insufficient permissions", errorResp.Message)
	})
}

func TestServer_GetCart(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration tests")
	}

	server, testDB, mockAuth := testCartServer(t)

	t.Run("successful get cart with items", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@get.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Get Cart Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item1 := testDB.NewItem(t).
			WithName("Item 1").
			WithType("medium").
			WithStock(10).
			Create()

		item2 := testDB.NewItem(t).
			WithName("Item 2").
			WithType("low").
			WithStock(50).
			Create()

		// Add items to cart
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item1.ID,
				Quantity: 2,
			},
		})
		require.NoError(t, err)

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		_, err = server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item2.ID,
				Quantity: 5,
			},
		})
		require.NoError(t, err)

		// Get cart
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		response, err := server.GetCart(ctx, api.GetCartRequestObject{
			GroupId: group.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetCart200JSONResponse{}, response)

		cartResp := response.(api.GetCart200JSONResponse)
		assert.Len(t, cartResp, 2)
	})

	t.Run("get empty cart returns empty array", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@empty.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Empty Cart Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetCart(ctx, api.GetCartRequestObject{
			GroupId: group.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetCart200JSONResponse{}, response)

		cartResp := response.(api.GetCart200JSONResponse)
		assert.Len(t, cartResp, 0)
		assert.NotNil(t, cartResp)
	})

	t.Run("user cannot view cart for group they are not member of", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@getnotmember.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Restricted Get Cart Group").
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetCart(ctx, api.GetCartRequestObject{
			GroupId: group.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetCart403JSONResponse{}, response)

		errorResp := response.(api.GetCart403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient permissions")
	})
}

func TestServer_UpdateCartItemQuantity(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration tests")
	}

	server, testDB, mockAuth := testCartServer(t)

	t.Run("successful update cart item quantity", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@updateqty.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Update Quantity Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Tripod").
			WithType("medium").
			WithStock(15).
			Create()

		// Add item to cart first
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item.ID,
				Quantity: 2,
			},
		})
		require.NoError(t, err)

		// Update quantity
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		response, err := server.UpdateCartItemQuantity(ctx, api.UpdateCartItemQuantityRequestObject{
			GroupId: group.ID,
			ItemId:  item.ID,
			Body: &api.UpdateCartItemQuantityJSONRequestBody{
				Quantity: 5,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.UpdateCartItemQuantity200JSONResponse{}, response)

		updateResp := response.(api.UpdateCartItemQuantity200JSONResponse)
		// 2 -> 5
		assert.Equal(t, 5, updateResp.Quantity)
	})

	t.Run("cannot update quantity to zero", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@updatezero.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Update Zero Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Light").
			WithType("medium").
			WithStock(8).
			Create()

		// Add item first
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item.ID,
				Quantity: 3,
			},
		})
		require.NoError(t, err)

		// update to zero
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		response, err := server.UpdateCartItemQuantity(ctx, api.UpdateCartItemQuantityRequestObject{
			GroupId: group.ID,
			ItemId:  item.ID,
			Body: &api.UpdateCartItemQuantityJSONRequestBody{
				Quantity: 0,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.UpdateCartItemQuantity400JSONResponse{}, response)

		errorResp := response.(api.UpdateCartItemQuantity400JSONResponse)
		assert.Equal(t, int32(400), errorResp.Code)
		assert.Contains(t, errorResp.Message, "greater than 0")
	})

	t.Run("user cannot update cart for group they are not member of", func(t *testing.T) {
		// User A adds item to their group's cart
		userA := testDB.NewUser(t).
			WithEmail("cart@userA.ca").
			AsMember().
			Create()

		groupA := testDB.NewGroup(t).
			WithName("Group A").
			Create()

		testDB.AssignUserToGroup(t, userA.ID, groupA.ID, "member")

		item := testDB.NewItem(t).
			WithName("Drone").
			WithType("high").
			WithStock(3).
			Create()

		mockAuth.ExpectCheckPermission(userA.ID, rbac.ManageCart, &groupA.ID, true, nil)
		ctxA := testutil.ContextWithUser(context.Background(), userA, testDB.Queries())

		_, err := server.AddToCart(ctxA, api.AddToCartRequestObject{
			GroupId: groupA.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  groupA.ID,
				ItemId:   item.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)

		// User B (not in Group A) tries to update
		userB := testDB.NewUser(t).
			WithEmail("cart@userB.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(userB.ID, rbac.ManageCart, &groupA.ID, false, nil)
		ctxB := testutil.ContextWithUser(context.Background(), userB, testDB.Queries())

		response, err := server.UpdateCartItemQuantity(ctxB, api.UpdateCartItemQuantityRequestObject{
			GroupId: groupA.ID,
			ItemId:  item.ID,
			Body: &api.UpdateCartItemQuantityJSONRequestBody{
				Quantity: 2,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.UpdateCartItemQuantity403JSONResponse{}, response)

		errorResp := response.(api.UpdateCartItemQuantity403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient permissions")
	})
}

func TestServer_RemoveFromCart(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration tests")
	}

	server, testDB, mockAuth := testCartServer(t)

	t.Run("successful remove item from cart", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@remove.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Remove Cart Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Cable").
			WithType("low").
			WithStock(100).
			Create()

		// Add item first
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item.ID,
				Quantity: 5,
			},
		})
		require.NoError(t, err)

		// Remove item
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		response, err := server.RemoveFromCart(ctx, api.RemoveFromCartRequestObject{
			GroupId: group.ID,
			ItemId:  item.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.RemoveFromCart204Response{}, response)

		// Verify item was removed
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		getResp, err := server.GetCart(ctx, api.GetCartRequestObject{
			GroupId: group.ID,
		})
		require.NoError(t, err)

		cartItems := getResp.(api.GetCart200JSONResponse)
		assert.Len(t, cartItems, 0)
	})

	t.Run("remove non-existent item from cart succeeds silently", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@removenotfound.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Remove Not Found Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.RemoveFromCart(ctx, api.RemoveFromCartRequestObject{
			GroupId: group.ID,
			ItemId:  uuid.New(),
		})

		// func is idempotent, removing empty or things that weren't there succeeds
		require.NoError(t, err)
		require.IsType(t, api.RemoveFromCart204Response{}, response)
	})

	t.Run("user cannot remove item from cart for group they are not member of", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@removenotmember.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Restricted Remove Group").
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.RemoveFromCart(ctx, api.RemoveFromCartRequestObject{
			GroupId: group.ID,
			ItemId:  uuid.New(),
		})

		require.NoError(t, err)
		require.IsType(t, api.RemoveFromCart403JSONResponse{}, response)

		errorResp := response.(api.RemoveFromCart403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient permissions")
	})
}

func TestServer_ClearCart(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration tests")
	}

	server, testDB, mockAuth := testCartServer(t)

	t.Run("successful clear cart with multiple items", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@clear.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Clear Cart Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item1 := testDB.NewItem(t).
			WithName("Item 1").
			WithType("medium").
			WithStock(10).
			Create()

		item2 := testDB.NewItem(t).
			WithName("Item 2").
			WithType("low").
			WithStock(20).
			Create()

		// Add items to cart
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item1.ID,
				Quantity: 2,
			},
		})
		require.NoError(t, err)

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		_, err = server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   item2.ID,
				Quantity: 3,
			},
		})
		require.NoError(t, err)

		// Clear cart
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		response, err := server.ClearCart(ctx, api.ClearCartRequestObject{
			GroupId: group.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.ClearCart204Response{}, response)

		// Verify cart is empty
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		getResp, err := server.GetCart(ctx, api.GetCartRequestObject{
			GroupId: group.ID,
		})
		require.NoError(t, err)

		cartItems := getResp.(api.GetCart200JSONResponse)
		assert.Len(t, cartItems, 0)
	})

	t.Run("clear empty cart succeeds", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@clearempty.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Clear Empty Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.ClearCart(ctx, api.ClearCartRequestObject{
			GroupId: group.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.ClearCart204Response{}, response)
	})

	t.Run("user cannot clear cart for group they are not member of", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("cart@clearnotmember.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Restricted Clear Group").
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.ClearCart(ctx, api.ClearCartRequestObject{
			GroupId: group.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.ClearCart403JSONResponse{}, response)

		errorResp := response.(api.ClearCart403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient permissions")
	})
}
