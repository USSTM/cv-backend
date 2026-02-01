package api

import (
	"context"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/internal/rbac"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_CheckoutCart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, mockAuth := newTestServer(t)

	t.Run("successful checkout with LOW items only", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("checkout@low.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Low Checkout Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		lowItem1 := testDB.NewItem(t).
			WithName("USB Cable").
			WithType("low").
			WithStock(100).
			Create()

		lowItem2 := testDB.NewItem(t).
			WithName("Battery").
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
				ItemId:   lowItem1.ID,
				Quantity: 10,
			},
		})
		require.NoError(t, err)

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		_, err = server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   lowItem2.ID,
				Quantity: 5,
			},
		})
		require.NoError(t, err)

		// Checkout
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		dueDate := time.Now().Add(7 * 24 * time.Hour)

		response, err := server.CheckoutCart(ctx, api.CheckoutCartRequestObject{
			Body: &api.CheckoutCartJSONRequestBody{
				GroupId: group.ID,
				DueDate: dueDate,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckoutCart200JSONResponse{}, response)

		checkoutResp := response.(api.CheckoutCart200JSONResponse)
		assert.Len(t, checkoutResp.LowItemsProcessed, 2)
		assert.Len(t, checkoutResp.MediumItemsBorrowed, 0)
		assert.Len(t, checkoutResp.HighItemsRequested, 0)
		assert.Len(t, checkoutResp.Errors, 0)

		// Verify stock decremented
		item1, err := testDB.Queries().GetItemByID(ctx, lowItem1.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(90), item1.Stock)

		item2, err := testDB.Queries().GetItemByID(ctx, lowItem2.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(45), item2.Stock)

		// Verify cart cleared
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		cartResp, err := server.GetCart(ctx, api.GetCartRequestObject{
			GroupId: group.ID,
		})
		require.NoError(t, err)
		cart := cartResp.(api.GetCart200JSONResponse)
		assert.Len(t, cart, 0)
	})

	t.Run("successful checkout with MEDIUM items only", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("checkout@medium.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Medium Checkout Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		mediumItem1 := testDB.NewItem(t).
			WithName("Projector").
			WithType("medium").
			WithStock(5).
			Create()

		mediumItem2 := testDB.NewItem(t).
			WithName("Camera").
			WithType("medium").
			WithStock(3).
			Create()

		// Add items to cart
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   mediumItem1.ID,
				Quantity: 2,
			},
		})
		require.NoError(t, err)

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		_, err = server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   mediumItem2.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)

		// Checkout
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := api.CheckoutCartRequestBeforeCondition("good")
		beforeConditionURL := "http://example.com/before.jpg"

		response, err := server.CheckoutCart(ctx, api.CheckoutCartRequestObject{
			Body: &api.CheckoutCartJSONRequestBody{
				GroupId:            group.ID,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckoutCart200JSONResponse{}, response)

		checkoutResp := response.(api.CheckoutCart200JSONResponse)
		assert.Len(t, checkoutResp.LowItemsProcessed, 0)
		assert.Len(t, checkoutResp.MediumItemsBorrowed, 2)
		assert.Len(t, checkoutResp.HighItemsRequested, 0)
		assert.Len(t, checkoutResp.Errors, 0)

		// Verify stock decremented
		item1, err := testDB.Queries().GetItemByID(ctx, mediumItem1.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(3), item1.Stock)

		item2, err := testDB.Queries().GetItemByID(ctx, mediumItem2.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(2), item2.Stock)
	})

	t.Run("successful checkout with HIGH items only", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("checkout@high.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("High Checkout Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		highItem1 := testDB.NewItem(t).
			WithName("Laptop").
			WithType("high").
			WithStock(5).
			Create()

		highItem2 := testDB.NewItem(t).
			WithName("Drone").
			WithType("high").
			WithStock(2).
			Create()

		// Add items to cart
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   highItem1.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		_, err = server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   highItem2.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)

		// Checkout
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)

		response, err := server.CheckoutCart(ctx, api.CheckoutCartRequestObject{
			Body: &api.CheckoutCartJSONRequestBody{
				GroupId: group.ID,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckoutCart200JSONResponse{}, response)

		checkoutResp := response.(api.CheckoutCart200JSONResponse)
		assert.Len(t, checkoutResp.LowItemsProcessed, 0)
		assert.Len(t, checkoutResp.MediumItemsBorrowed, 0)
		assert.Len(t, checkoutResp.HighItemsRequested, 2)
		assert.Len(t, checkoutResp.Errors, 0)

		// Verify stock not decremented
		item1, err := testDB.Queries().GetItemByID(ctx, highItem1.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(5), item1.Stock)

		item2, err := testDB.Queries().GetItemByID(ctx, highItem2.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(2), item2.Stock)
	})

	t.Run("successful checkout with mixed item types", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("checkout@mixed.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Mixed Checkout Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		lowItem := testDB.NewItem(t).
			WithName("Adapter").
			WithType("low").
			WithStock(100).
			Create()

		mediumItem := testDB.NewItem(t).
			WithName("Tripod").
			WithType("medium").
			WithStock(10).
			Create()

		highItem := testDB.NewItem(t).
			WithName("MacBook").
			WithType("high").
			WithStock(3).
			Create()

		// Add mixed items to cart
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   lowItem.ID,
				Quantity: 5,
			},
		})
		require.NoError(t, err)

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		_, err = server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   mediumItem.ID,
				Quantity: 2,
			},
		})
		require.NoError(t, err)

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		_, err = server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)

		// Checkout
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := api.CheckoutCartRequestBeforeCondition("good")
		beforeConditionURL := "http://example.com/before.jpg"

		response, err := server.CheckoutCart(ctx, api.CheckoutCartRequestObject{
			Body: &api.CheckoutCartJSONRequestBody{
				GroupId:            group.ID,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckoutCart200JSONResponse{}, response)

		checkoutResp := response.(api.CheckoutCart200JSONResponse)
		assert.Len(t, checkoutResp.LowItemsProcessed, 1)
		assert.Len(t, checkoutResp.MediumItemsBorrowed, 1)
		assert.Len(t, checkoutResp.HighItemsRequested, 1)
		assert.Len(t, checkoutResp.Errors, 0)
	})

	t.Run("checkout with insufficient stock for LOW item returns error", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("checkout@lownostock.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Low No Stock Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		lowItem := testDB.NewItem(t).
			WithName("Limited Cable").
			WithType("low").
			WithStock(5).
			Create()

		// Add more than available to cart
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   lowItem.ID,
				Quantity: 10,
			},
		})
		require.NoError(t, err)

		// Checkout should error for this item
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)

		response, err := server.CheckoutCart(ctx, api.CheckoutCartRequestObject{
			Body: &api.CheckoutCartJSONRequestBody{
				GroupId: group.ID,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckoutCart200JSONResponse{}, response)

		checkoutResp := response.(api.CheckoutCart200JSONResponse)
		assert.Len(t, checkoutResp.Errors, 1)
		assert.Contains(t, checkoutResp.Errors[0].Message, "insufficient stock")
	})

	t.Run("checkout with insufficient stock for MEDIUM item returns error", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("checkout@mediumnostock.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Medium No Stock Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		mediumItem := testDB.NewItem(t).
			WithName("Limited Camera").
			WithType("medium").
			WithStock(2).
			Create()

		// Add more than available to cart
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   mediumItem.ID,
				Quantity: 5,
			},
		})
		require.NoError(t, err)

		// Checkout should return error for item
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := api.CheckoutCartRequestBeforeCondition("good")
		beforeConditionURL := "http://example.com/before.jpg"

		response, err := server.CheckoutCart(ctx, api.CheckoutCartRequestObject{
			Body: &api.CheckoutCartJSONRequestBody{
				GroupId:            group.ID,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckoutCart200JSONResponse{}, response)

		checkoutResp := response.(api.CheckoutCart200JSONResponse)
		assert.Len(t, checkoutResp.Errors, 1)
		assert.Contains(t, checkoutResp.Errors[0].Message, "insufficient stock")
	})

	t.Run("cannot checkout empty cart", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("checkout@empty.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Empty Checkout Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.CheckoutCart(ctx, api.CheckoutCartRequestObject{
			Body: &api.CheckoutCartJSONRequestBody{
				GroupId: group.ID,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckoutCart400JSONResponse{}, response)

		errorResp := response.(api.CheckoutCart400JSONResponse)
		assert.Equal(t, "VALIDATION_ERROR", string(errorResp.Error.Code))
		assert.Contains(t, errorResp.Error.Message, "empty")
	})

	t.Run("user cannot checkout cart for group they are not member of", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("checkout@notmember.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Restricted Checkout Group").
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.CheckoutCart(ctx, api.CheckoutCartRequestObject{
			Body: &api.CheckoutCartJSONRequestBody{
				GroupId: group.ID,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckoutCart403JSONResponse{}, response)

		errorResp := response.(api.CheckoutCart403JSONResponse)
		assert.Equal(t, "PERMISSION_DENIED", string(errorResp.Error.Code))
		assert.Contains(t, errorResp.Error.Message, "Insufficient permissions")
	})

	t.Run("checkout with partial success some succeed, some fail", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("checkout@partial.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Partial Success Group").
			Create()

		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		// Good item with sufficient stock
		goodItem := testDB.NewItem(t).
			WithName("Good Item").
			WithType("low").
			WithStock(100).
			Create()

		// Bad item with insufficient stock
		badItem := testDB.NewItem(t).
			WithName("Bad Item").
			WithType("low").
			WithStock(1).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   goodItem.ID,
				Quantity: 10,
			},
		})
		require.NoError(t, err)

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		_, err = server.AddToCart(ctx, api.AddToCartRequestObject{
			GroupId: group.ID,
			Body: &api.AddToCartJSONRequestBody{
				GroupId:  group.ID,
				ItemId:   badItem.ID,
				Quantity: 10, // More than available
			},
		})
		require.NoError(t, err)

		// Checkout
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)

		response, err := server.CheckoutCart(ctx, api.CheckoutCartRequestObject{
			Body: &api.CheckoutCartJSONRequestBody{
				GroupId: group.ID,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckoutCart200JSONResponse{}, response)

		checkoutResp := response.(api.CheckoutCart200JSONResponse)
		assert.Len(t, checkoutResp.LowItemsProcessed, 1) // Good item succeeded
		assert.Len(t, checkoutResp.Errors, 1)            // Bad item failed

		// Verify cart is cleared even with partial failure
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageCart, &group.ID, true, nil)
		cartResp, err := server.GetCart(ctx, api.GetCartRequestObject{
			GroupId: group.ID,
		})
		require.NoError(t, err)
		cart := cartResp.(api.GetCart200JSONResponse)
		assert.Len(t, cart, 0)
	})
}
