package api

import (
	"github.com/USSTM/cv-backend/internal/rbac"
	"context"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testBorrowingServer(t *testing.T) (*Server, *testutil.TestDatabase, *testutil.MockAuthenticator) {
	testDB := getSharedTestDatabase(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)

	server := NewServer(testDB, mockJWT, mockAuth)

	return server, testDB, mockAuth
}

func TestServer_BorrowItem(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration tests")
	}

	server, testDB, mockAuth := testBorrowingServer(t)

	t.Run("successful borrow of medium-type item by member", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("borrow@medium.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Test Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Projector").
			WithDescription("Epson HD Projector").
			WithType("medium").
			WithStock(5).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		response, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           2,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.BorrowItem201JSONResponse{}, response)

		borrowResp := response.(api.BorrowItem201JSONResponse)
		assert.NotEqual(t, uuid.Nil, borrowResp.Id)
		assert.Equal(t, testUser.ID, borrowResp.UserId)
		assert.Equal(t, item.ID, borrowResp.ItemId)
		assert.Equal(t, 2, borrowResp.Quantity)
		assert.Nil(t, borrowResp.ReturnedAt)
		assert.Equal(t, beforeCondition, borrowResp.BeforeCondition)
		assert.Equal(t, beforeConditionURL, borrowResp.BeforeConditionUrl)

		// Verify stock was decremented
		updatedItem, err := testDB.Queries().GetItemByID(ctx, item.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(3), updatedItem.Stock, "Stock should be decremented from 5 to 3 after borrowing 2 items")
	})

	t.Run("attempt to borrow already borrowed item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("borrow@conflict.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Conflict Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Camera").
			WithDescription("Canon EOS").
			WithType("medium").
			WithStock(1).
			Create()

		// First borrow
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		_, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})
		require.NoError(t, err)

		// Try to borrow again
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)

		response, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.BorrowItem400JSONResponse{}, response)

		errorResp := response.(api.BorrowItem400JSONResponse)
		assert.Equal(t, int32(400), errorResp.Code)
		// For MEDIUM items with stock-based tracking, when stock is depleted it says "Insufficient stock"
		assert.Contains(t, errorResp.Message, "Insufficient stock")
	})

	t.Run("attempt to borrow without permission", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("borrow@noperm.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("No Perm Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Tablet").
			WithDescription("iPad").
			WithType("medium").
			WithStock(5).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		response, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.BorrowItem403JSONResponse{}, response)

		errorResp := response.(api.BorrowItem403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Equal(t, "Insufficient permissions", errorResp.Message)
	})

	t.Run("attempt to borrow non-existent item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("borrow@notfound.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("NotFound Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		nonExistentItemID := uuid.New()
		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		response, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             nonExistentItemID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.BorrowItem500JSONResponse{}, response)
	})

	t.Run("attempt to borrow high-value item without approved request", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("borrow@highnonapproved.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("High No Request Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Professional Camera").
			WithDescription("Sony A7III").
			WithType("high").
			WithStock(2).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "excellent"
		beforeConditionURL := "http://example.com/camera-before.jpg"

		response, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.BorrowItem403JSONResponse{}, response)

		errorResp := response.(api.BorrowItem403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "approved request")
	})

	t.Run("attempt to borrow with insufficient stock", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("borrow@insufficientstock.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Insufficient Stock Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Microphone").
			WithDescription("USB Microphone").
			WithType("medium").
			WithStock(2).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "new"
		beforeConditionURL := "http://example.com/mic-before.jpg"

		response, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           5, // Requesting more than available stock (2)
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.BorrowItem400JSONResponse{}, response)

		errorResp := response.(api.BorrowItem400JSONResponse)
		assert.Equal(t, int32(400), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient stock")
	})

	t.Run("user cannot borrow item for group they are not member of", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("borrow@notmember.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Restricted Group").
			Create()

		// NOTE: Intentionally NOT calling AssignUserToGroup to test security

		item := testDB.NewItem(t).
			WithName("Laptop").
			WithDescription("MacBook Pro").
			WithType("medium").
			WithStock(5).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		response, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.BorrowItem403JSONResponse{}, response)

		errorResp := response.(api.BorrowItem403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient permissions")
	})
}

func TestServer_ReturnItem(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping return item tests in short mode")
	}

	server, testDB, mockAuth := testBorrowingServer(t)

	t.Run("successful return of borrowed item with after condition", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("return@success.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Return Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Microphone").
			WithDescription("Shure SM58").
			WithType("medium").
			WithStock(5).
			Create()

		// First borrow the item
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		borrowResp, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})
		require.NoError(t, err)
		require.IsType(t, api.BorrowItem201JSONResponse{}, borrowResp)

		// Verify stock was decremented after borrow
		itemAfterBorrow, err := testDB.Queries().GetItemByID(ctx, item.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(4), itemAfterBorrow.Stock, "Stock should be 4 after borrowing 1 item")

		// Now return the item
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)

		afterCondition := "decent"
		afterConditionURL := "http://example.com/after.jpg"

		response, err := server.ReturnItem(ctx, api.ReturnItemRequestObject{
			ItemId: item.ID,
			Body: &api.ReturnItemJSONRequestBody{
				AfterCondition:    afterCondition,
				AfterConditionUrl: &afterConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ReturnItem200JSONResponse{}, response)

		returnResp := response.(api.ReturnItem200JSONResponse)
		assert.Equal(t, item.ID, returnResp.ItemId)
		assert.NotNil(t, returnResp.ReturnedAt)
		assert.Equal(t, &afterCondition, returnResp.AfterCondition)
		assert.Equal(t, &afterConditionURL, returnResp.AfterConditionUrl)

		// Verify stock was incremented after return
		itemAfterReturn, err := testDB.Queries().GetItemByID(ctx, item.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(5), itemAfterReturn.Stock, "Stock should be back to 5 after returning 1 item")
	})

	t.Run("attempt to return non-borrowed item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("return@notborrowed.ca").
			AsMember().
			Create()

		item := testDB.NewItem(t).
			WithName("Headphones").
			WithDescription("Sony WH-1000XM4").
			WithType("medium").
			WithStock(10).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		afterCondition := "good"
		afterConditionURL := "http://example.com/after.jpg"

		response, err := server.ReturnItem(ctx, api.ReturnItemRequestObject{
			ItemId: item.ID,
			Body: &api.ReturnItemJSONRequestBody{
				AfterCondition:    afterCondition,
				AfterConditionUrl: &afterConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ReturnItem403JSONResponse{}, response)

		errorResp := response.(api.ReturnItem403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "not actively borrowed by you")
	})

	t.Run("attempt to return without permission", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("return@noperm.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("No Permission Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Speaker").
			WithDescription("JBL Charge 5").
			WithType("medium").
			WithStock(3).
			Create()

		// First borrow
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		_, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})
		require.NoError(t, err)

		// Try to return without permission
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, false, nil)

		afterCondition := "good"
		afterConditionURL := "http://example.com/after.jpg"

		response, err := server.ReturnItem(ctx, api.ReturnItemRequestObject{
			ItemId: item.ID,
			Body: &api.ReturnItemJSONRequestBody{
				AfterCondition:    afterCondition,
				AfterConditionUrl: &afterConditionURL,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ReturnItem403JSONResponse{}, response)

		errorResp := response.(api.ReturnItem403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Equal(t, "Insufficient permissions", errorResp.Message)
	})
}

func TestServer_CheckBorrowingItemStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping check borrowing item status tests in short mode")
	}

	server, testDB, mockAuth := testBorrowingServer(t)

	t.Run("check status of available item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("status@available.ca").
			AsMember().
			Create()

		item := testDB.NewItem(t).
			WithName("Monitor").
			WithDescription("Dell 27 inch").
			WithType("medium").
			WithStock(10).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.CheckBorrowingItemStatus(ctx, api.CheckBorrowingItemStatusRequestObject{
			ItemId: item.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckBorrowingItemStatus200JSONResponse{}, response)

		statusResp := response.(api.CheckBorrowingItemStatus200JSONResponse)
		assert.NotNil(t, statusResp.IsBorrowed)
		assert.True(t, *statusResp.IsBorrowed) // Item is available (not borrowed)
	})

	t.Run("check status of borrowed item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("status@borrowed.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Status Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Keyboard").
			WithDescription("Mechanical").
			WithType("medium").
			WithStock(1).
			Create()

		// Borrow the item first
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		_, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})
		require.NoError(t, err)

		// Now check status
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, nil, true, nil)

		response, err := server.CheckBorrowingItemStatus(ctx, api.CheckBorrowingItemStatusRequestObject{
			ItemId: item.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckBorrowingItemStatus200JSONResponse{}, response)

		statusResp := response.(api.CheckBorrowingItemStatus200JSONResponse)
		assert.NotNil(t, statusResp.IsBorrowed)
		assert.False(t, *statusResp.IsBorrowed) // Item is not available (borrowed)
	})

	t.Run("check status without permission", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("status@noperm.ca").
			AsMember().
			Create()

		item := testDB.NewItem(t).
			WithName("Mouse").
			WithDescription("Logitech MX Master").
			WithType("low").
			WithStock(5).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.CheckBorrowingItemStatus(ctx, api.CheckBorrowingItemStatusRequestObject{
			ItemId: item.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.CheckBorrowingItemStatus403JSONResponse{}, response)

		errorResp := response.(api.CheckBorrowingItemStatus403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Equal(t, "Insufficient permissions", errorResp.Message)
	})
}

func TestServer_UserBorrowingHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping user borrowing history tests in short mode")
	}

	server, testDB, mockAuth := testBorrowingServer(t)

	t.Run("user views their own full history", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("history@own.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("History Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item1 := testDB.NewItem(t).
			WithName("Item 1").
			WithType("medium").
			WithStock(5).
			Create()

		item2 := testDB.NewItem(t).
			WithName("Item 2").
			WithType("medium").
			WithStock(5).
			Create()

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		// Borrow two items
		for _, item := range []struct{ id uuid.UUID }{
			{item1.ID},
			{item2.ID},
		} {
			mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
			dueDate := time.Now().Add(7 * 24 * time.Hour)
			beforeCondition := "good"
			beforeConditionURL := "http://example.com/before.jpg"

			_, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
				Body: &api.BorrowItemJSONRequestBody{
					UserId:             testUser.ID,
					GroupId:            group.ID,
					ItemId:             item.id,
					Quantity:           1,
					DueDate:            dueDate,
					BeforeCondition:    beforeCondition,
					BeforeConditionUrl: beforeConditionURL,
				},
			})
			require.NoError(t, err)
		}

		// Return one item
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)
		afterCondition := "good"
		afterConditionURL := "http://example.com/after.jpg"

		_, err := server.ReturnItem(ctx, api.ReturnItemRequestObject{
			ItemId: item1.ID,
			Body: &api.ReturnItemJSONRequestBody{
				AfterCondition:    afterCondition,
				AfterConditionUrl: &afterConditionURL,
			},
		})
		require.NoError(t, err)

		// Get full history
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)

		response, err := server.GetBorrowedItemHistoryByUserId(ctx, api.GetBorrowedItemHistoryByUserIdRequestObject{
			UserId: testUser.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetBorrowedItemHistoryByUserId200JSONResponse{}, response)

		historyResp := response.(api.GetBorrowedItemHistoryByUserId200JSONResponse)
		assert.Len(t, historyResp, 2) // Should have 2 borrowings (1 returned, 1 active)
	})

	t.Run("user attempts to view another user's history", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("history@unauthorized.ca").
			AsMember().
			Create()

		otherUser := testDB.NewUser(t).
			WithEmail("history@other.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetBorrowedItemHistoryByUserId(ctx, api.GetBorrowedItemHistoryByUserIdRequestObject{
			UserId: otherUser.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetBorrowedItemHistoryByUserId403JSONResponse{}, response)

		errorResp := response.(api.GetBorrowedItemHistoryByUserId403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "view other users")
	})

	t.Run("user views their own active borrowings", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("active@own.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Active Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Active Item").
			WithType("medium").
			WithStock(5).
			Create()

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		// Borrow item
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		_, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})
		require.NoError(t, err)

		// Get active borrowings
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)

		response, err := server.GetActiveBorrowedItemsByUserId(ctx, api.GetActiveBorrowedItemsByUserIdRequestObject{
			UserId: testUser.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetActiveBorrowedItemsByUserId200JSONResponse{}, response)

		activeResp := response.(api.GetActiveBorrowedItemsByUserId200JSONResponse)
		assert.Len(t, activeResp, 1)
		assert.Nil(t, activeResp[0].ReturnedAt)
	})

	t.Run("user views their own returned items", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("returned@own.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Returned Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Returned Item").
			WithType("medium").
			WithStock(5).
			Create()

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		// Borrow and return item
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		_, err := server.BorrowItem(ctx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})
		require.NoError(t, err)

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)
		afterCondition := "good"
		afterConditionURL := "http://example.com/after.jpg"

		_, err = server.ReturnItem(ctx, api.ReturnItemRequestObject{
			ItemId: item.ID,
			Body: &api.ReturnItemJSONRequestBody{
				AfterCondition:    afterCondition,
				AfterConditionUrl: &afterConditionURL,
			},
		})
		require.NoError(t, err)

		// Get returned items
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)

		response, err := server.GetReturnedItemsByUserId(ctx, api.GetReturnedItemsByUserIdRequestObject{
			UserId: testUser.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetReturnedItemsByUserId200JSONResponse{}, response)

		returnedResp := response.(api.GetReturnedItemsByUserId200JSONResponse)
		assert.Len(t, returnedResp, 1)
		assert.NotNil(t, returnedResp[0].ReturnedAt)
	})
}

func TestServer_AdminBorrowingViews(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping admin borrowing view tests in short mode")
	}

	server, testDB, mockAuth := testBorrowingServer(t)

	t.Run("admin views all active borrowings", func(t *testing.T) {
		adminUser := testDB.NewUser(t).
			WithEmail("admin@allactive.ca").
			AsGlobalAdmin().
			Create()

		testUser := testDB.NewUser(t).
			WithEmail("member@allactive.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Admin View Group").
			Create()

		// Assign member to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Admin Item").
			WithType("medium").
			WithStock(5).
			Create()

		// Member borrows item
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		memberCtx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		dueDate := time.Now().Add(7 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		_, err := server.BorrowItem(memberCtx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})
		require.NoError(t, err)

		// Admin views all active
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ViewAllData, nil, true, nil)
		adminCtx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		response, err := server.GetAllActiveBorrowedItems(adminCtx, api.GetAllActiveBorrowedItemsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetAllActiveBorrowedItems200JSONResponse{}, response)

		activeResp := response.(api.GetAllActiveBorrowedItems200JSONResponse)
		assert.GreaterOrEqual(t, len(activeResp), 1)
	})

	t.Run("member attempts to view all borrowings", func(t *testing.T) {
		memberUser := testDB.NewUser(t).
			WithEmail("member@unauthorized.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(memberUser.ID, rbac.ViewAllData, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), memberUser, testDB.Queries())

		response, err := server.GetAllActiveBorrowedItems(ctx, api.GetAllActiveBorrowedItemsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetAllActiveBorrowedItems403JSONResponse{}, response)

		errorResp := response.(api.GetAllActiveBorrowedItems403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Equal(t, "Insufficient permissions", errorResp.Message)
	})

	t.Run("admin views all returned items", func(t *testing.T) {
		adminUser := testDB.NewUser(t).
			WithEmail("admin@returned.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ViewAllData, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		response, err := server.GetAllReturnedItems(ctx, api.GetAllReturnedItemsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetAllReturnedItems200JSONResponse{}, response)

		// Response may be empty or have items depending on previous tests
		_ = response.(api.GetAllReturnedItems200JSONResponse)
	})

	t.Run("admin views borrowings due by date", func(t *testing.T) {
		adminUser := testDB.NewUser(t).
			WithEmail("admin@duedate.ca").
			AsGlobalAdmin().
			Create()

		testUser := testDB.NewUser(t).
			WithEmail("member@duedate.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Due Date Group").
			Create()

		// Assign member to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		item := testDB.NewItem(t).
			WithName("Due Date Item").
			WithType("medium").
			WithStock(5).
			Create()

		// Member borrows item with specific due date
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		memberCtx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		dueDate := time.Now().Add(3 * 24 * time.Hour)
		beforeCondition := "good"
		beforeConditionURL := "http://example.com/before.jpg"

		_, err := server.BorrowItem(memberCtx, api.BorrowItemRequestObject{
			Body: &api.BorrowItemJSONRequestBody{
				UserId:             testUser.ID,
				GroupId:            group.ID,
				ItemId:             item.ID,
				Quantity:           1,
				DueDate:            dueDate,
				BeforeCondition:    beforeCondition,
				BeforeConditionUrl: beforeConditionURL,
			},
		})
		require.NoError(t, err)

		// Admin views items due by a future date
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ViewAllData, nil, true, nil)
		adminCtx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		futureDate := time.Now().Add(7 * 24 * time.Hour)

		response, err := server.GetActiveBorrowedItemsToBeReturnedByDate(adminCtx, api.GetActiveBorrowedItemsToBeReturnedByDateRequestObject{
			DueDate: openapi_types.Date{Time: futureDate},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetActiveBorrowedItemsToBeReturnedByDate200JSONResponse{}, response)

		dueDateResp := response.(api.GetActiveBorrowedItemsToBeReturnedByDate200JSONResponse)
		assert.GreaterOrEqual(t, len(dueDateResp), 1)
	})
}

func TestServer_RequestItem(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping request item tests in short mode")
	}

	server, testDB, mockAuth := testBorrowingServer(t)

	t.Run("successful request for high-value item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("request@high.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Request Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("Laptop").
			WithType("high").
			WithStock(3).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.RequestItem(ctx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   testUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.RequestItem201JSONResponse{}, response)

		requestResp := response.(api.RequestItem201JSONResponse)
		assert.NotEqual(t, uuid.Nil, requestResp.Id)
		assert.Equal(t, testUser.ID, requestResp.UserId)
		assert.Equal(t, group.ID, requestResp.GroupId)
		assert.Equal(t, highItem.ID, requestResp.ItemId)
		assert.Equal(t, 1, requestResp.Quantity)
		assert.Equal(t, api.Pending, requestResp.Status)
		assert.Nil(t, requestResp.ReviewedBy)
		assert.Nil(t, requestResp.ReviewedAt)
	})

	t.Run("attempt to request low-value item returns error", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("request@low.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Low Request Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		lowItem := testDB.NewItem(t).
			WithName("Cable").
			WithType("low").
			WithStock(10).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.RequestItem(ctx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   testUser.ID,
				GroupId:  group.ID,
				ItemId:   lowItem.ID,
				Quantity: 1,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.RequestItem400JSONResponse{}, response)

		errorResp := response.(api.RequestItem400JSONResponse)
		assert.Equal(t, int32(400), errorResp.Code)
		assert.Contains(t, errorResp.Message, "high-value items")
	})

	t.Run("attempt to request non-existent item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("request@notfound.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Not Found Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.RequestItem(ctx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   testUser.ID,
				GroupId:  group.ID,
				ItemId:   uuid.New(),
				Quantity: 1,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.RequestItem404JSONResponse{}, response)

		errorResp := response.(api.RequestItem404JSONResponse)
		assert.Equal(t, int32(404), errorResp.Code)
		assert.Contains(t, errorResp.Message, "not found")
	})

	t.Run("user without permission cannot request item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("request@noperm.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("No Perm Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("Expensive Camera").
			WithType("high").
			WithStock(2).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.RequestItem(ctx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   testUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.RequestItem403JSONResponse{}, response)

		errorResp := response.(api.RequestItem403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
	})

	t.Run("user cannot request item for group they are not member of", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("request@notmember.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Restricted Request Group").
			Create()

		// NOTE: Intentionally NOT calling AssignUserToGroup to test security

		highItem := testDB.NewItem(t).
			WithName("Professional Drone").
			WithType("high").
			WithStock(3).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.RequestItem(ctx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   testUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.RequestItem403JSONResponse{}, response)

		errorResp := response.(api.RequestItem403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient permissions")
	})
}

func TestServer_ReviewRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping review request tests in short mode")
	}

	server, testDB, mockAuth := testBorrowingServer(t)

	t.Run("approver successfully approves request", func(t *testing.T) {
		requestUser := testDB.NewUser(t).
			WithEmail("requester@approve.ca").
			AsMember().
			Create()

		approverUser := testDB.NewUser(t).
			WithEmail("approver@approve.ca").
			AsApprover().
			Create()

		group := testDB.NewGroup(t).
			WithName("Approve Group").
			Create()

		// Assign requester to group
		testDB.AssignUserToGroup(t, requestUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("DSLR Camera").
			WithType("high").
			WithStock(2).
			Create()

		// Create request context
		requestCtx := testutil.ContextWithUser(context.Background(), requestUser, testDB.Queries())

		// Get a time slot from seed data
		timeSlots, err := testDB.Queries().ListTimeSlots(requestCtx)
		require.NoError(t, err)
		require.NotEmpty(t, timeSlots)
		timeSlotID := timeSlots[0].ID

		// Create availability for booking
		futureDate := time.Now().Add(24 * time.Hour) // Tomorrow
		availability, err := testDB.Queries().CreateAvailability(requestCtx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &requestUser.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create request
		mockAuth.ExpectCheckPermission(requestUser.ID, rbac.RequestItems, &group.ID, true, nil)

		requestResp, err := server.RequestItem(requestCtx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   requestUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)
		createdRequest := requestResp.(api.RequestItem201JSONResponse)

		// Approve request with booking fields
		mockAuth.ExpectCheckPermission(approverUser.ID, rbac.ApproveAllRequests, nil, true, nil)
		approverCtx := testutil.ContextWithUser(context.Background(), approverUser, testDB.Queries())

		pickupLocation := "Main Office"
		returnLocation := "Equipment Room"

		response, err := server.ReviewRequest(approverCtx, api.ReviewRequestRequestObject{
			RequestId: createdRequest.Id,
			Body: &api.ReviewRequestJSONRequestBody{
				Status:         api.Approved,
				AvailabilityId: &availability.ID,
				PickupLocation: &pickupLocation,
				ReturnLocation: &returnLocation,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ReviewRequest200JSONResponse{}, response)

		reviewResp := response.(api.ReviewRequest200JSONResponse)
		assert.Equal(t, createdRequest.Id, reviewResp.Id)
		assert.Equal(t, api.Approved, reviewResp.Status)
		assert.Equal(t, approverUser.ID, *reviewResp.ReviewedBy)
		assert.NotNil(t, reviewResp.ReviewedAt)
	})

	t.Run("approver denies request", func(t *testing.T) {
		requestUser := testDB.NewUser(t).
			WithEmail("requester@deny.ca").
			AsMember().
			Create()

		approverUser := testDB.NewUser(t).
			WithEmail("approver@deny.ca").
			AsApprover().
			Create()

		group := testDB.NewGroup(t).
			WithName("Deny Group").
			Create()

		// Assign requester to group
		testDB.AssignUserToGroup(t, requestUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("Video Camera").
			WithType("high").
			WithStock(1).
			Create()

		// Create request
		mockAuth.ExpectCheckPermission(requestUser.ID, rbac.RequestItems, &group.ID, true, nil)
		requestCtx := testutil.ContextWithUser(context.Background(), requestUser, testDB.Queries())

		requestResp, err := server.RequestItem(requestCtx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   requestUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)
		createdRequest := requestResp.(api.RequestItem201JSONResponse)

		// Deny request
		mockAuth.ExpectCheckPermission(approverUser.ID, rbac.ApproveAllRequests, nil, true, nil)
		approverCtx := testutil.ContextWithUser(context.Background(), approverUser, testDB.Queries())

		response, err := server.ReviewRequest(approverCtx, api.ReviewRequestRequestObject{
			RequestId: createdRequest.Id,
			Body: &api.ReviewRequestJSONRequestBody{
				Status: api.Denied,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ReviewRequest200JSONResponse{}, response)

		reviewResp := response.(api.ReviewRequest200JSONResponse)
		assert.Equal(t, api.Denied, reviewResp.Status)
		assert.Equal(t, approverUser.ID, *reviewResp.ReviewedBy)
	})

	t.Run("cannot approve request with insufficient stock", func(t *testing.T) {
		requestUser := testDB.NewUser(t).
			WithEmail("requester@nostock.ca").
			AsMember().
			Create()

		approverUser := testDB.NewUser(t).
			WithEmail("approver@nostock.ca").
			AsApprover().
			Create()

		group := testDB.NewGroup(t).
			WithName("No Stock Group").
			Create()

		// Assign requester to group
		testDB.AssignUserToGroup(t, requestUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("Drone").
			WithType("high").
			WithStock(0). // No stock available
			Create()

		// Create request
		mockAuth.ExpectCheckPermission(requestUser.ID, rbac.RequestItems, &group.ID, true, nil)
		requestCtx := testutil.ContextWithUser(context.Background(), requestUser, testDB.Queries())

		requestResp, err := server.RequestItem(requestCtx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   requestUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)
		createdRequest := requestResp.(api.RequestItem201JSONResponse)

		// Try to approve request
		mockAuth.ExpectCheckPermission(approverUser.ID, rbac.ApproveAllRequests, nil, true, nil)
		approverCtx := testutil.ContextWithUser(context.Background(), approverUser, testDB.Queries())

		response, err := server.ReviewRequest(approverCtx, api.ReviewRequestRequestObject{
			RequestId: createdRequest.Id,
			Body: &api.ReviewRequestJSONRequestBody{
				Status: api.Approved,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ReviewRequest400JSONResponse{}, response)

		errorResp := response.(api.ReviewRequest400JSONResponse)
		assert.Equal(t, int32(400), errorResp.Code)
		assert.Contains(t, errorResp.Message, "stock")
	})

	t.Run("member cannot review request", func(t *testing.T) {
		requestUser := testDB.NewUser(t).
			WithEmail("requester@memberapprove.ca").
			AsMember().
			Create()

		memberUser := testDB.NewUser(t).
			WithEmail("member@noapprove.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Member Approve Group").
			Create()

		// Assign requester to group
		testDB.AssignUserToGroup(t, requestUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("Gimbal").
			WithType("high").
			WithStock(1).
			Create()

		// Create request
		mockAuth.ExpectCheckPermission(requestUser.ID, rbac.RequestItems, &group.ID, true, nil)
		requestCtx := testutil.ContextWithUser(context.Background(), requestUser, testDB.Queries())

		requestResp, err := server.RequestItem(requestCtx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   requestUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)
		createdRequest := requestResp.(api.RequestItem201JSONResponse)

		// Member tries to approve
		mockAuth.ExpectCheckPermission(memberUser.ID, rbac.ApproveAllRequests, nil, false, nil)
		memberCtx := testutil.ContextWithUser(context.Background(), memberUser, testDB.Queries())

		response, err := server.ReviewRequest(memberCtx, api.ReviewRequestRequestObject{
			RequestId: createdRequest.Id,
			Body: &api.ReviewRequestJSONRequestBody{
				Status: api.Approved,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ReviewRequest403JSONResponse{}, response)

		errorResp := response.(api.ReviewRequest403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
	})

	t.Run("cannot review already reviewed request", func(t *testing.T) {
		requestUser := testDB.NewUser(t).
			WithEmail("requester@double.ca").
			AsMember().
			Create()

		approverUser := testDB.NewUser(t).
			WithEmail("approver@double.ca").
			AsApprover().
			Create()

		group := testDB.NewGroup(t).
			WithName("Double Review Group").
			Create()

		// Assign requester to group
		testDB.AssignUserToGroup(t, requestUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("Microphone").
			WithType("high").
			WithStock(2).
			Create()

		// Create request context
		requestCtx := testutil.ContextWithUser(context.Background(), requestUser, testDB.Queries())

		// Get a time slot from seed data
		timeSlots, err := testDB.Queries().ListTimeSlots(requestCtx)
		require.NoError(t, err)
		require.NotEmpty(t, timeSlots)
		timeSlotID := timeSlots[0].ID

		// Create availability for booking
		futureDate := time.Now().Add(48 * time.Hour) // 2 days from now
		availability, err := testDB.Queries().CreateAvailability(requestCtx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &requestUser.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create request
		mockAuth.ExpectCheckPermission(requestUser.ID, rbac.RequestItems, &group.ID, true, nil)

		requestResp, err := server.RequestItem(requestCtx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   requestUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)
		createdRequest := requestResp.(api.RequestItem201JSONResponse)

		// First approval with booking fields
		mockAuth.ExpectCheckPermission(approverUser.ID, rbac.ApproveAllRequests, nil, true, nil)
		approverCtx := testutil.ContextWithUser(context.Background(), approverUser, testDB.Queries())

		pickupLocation := "Main Office"
		returnLocation := "Equipment Room"

		_, err = server.ReviewRequest(approverCtx, api.ReviewRequestRequestObject{
			RequestId: createdRequest.Id,
			Body: &api.ReviewRequestJSONRequestBody{
				Status:         api.Approved,
				AvailabilityId: &availability.ID,
				PickupLocation: &pickupLocation,
				ReturnLocation: &returnLocation,
			},
		})
		require.NoError(t, err)

		// Try to review again
		mockAuth.ExpectCheckPermission(approverUser.ID, rbac.ApproveAllRequests, nil, true, nil)

		response, err := server.ReviewRequest(approverCtx, api.ReviewRequestRequestObject{
			RequestId: createdRequest.Id,
			Body: &api.ReviewRequestJSONRequestBody{
				Status: api.Denied,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ReviewRequest400JSONResponse{}, response)

		errorResp := response.(api.ReviewRequest400JSONResponse)
		assert.Equal(t, int32(400), errorResp.Code)
		assert.Contains(t, errorResp.Message, "already reviewed")
	})
}

func TestServer_GetAllRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping get all requests tests in short mode")
	}

	server, testDB, mockAuth := testBorrowingServer(t)

	t.Run("admin views all requests", func(t *testing.T) {
		adminUser := testDB.NewUser(t).
			WithEmail("admin@allrequests.ca").
			AsGlobalAdmin().
			Create()

		requestUser := testDB.NewUser(t).
			WithEmail("requester@allrequests.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("All Requests Group").
			Create()

		// Assign requester to group
		testDB.AssignUserToGroup(t, requestUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("MacBook Pro").
			WithType("high").
			WithStock(1).
			Create()

		// Create a request
		mockAuth.ExpectCheckPermission(requestUser.ID, rbac.RequestItems, &group.ID, true, nil)
		requestCtx := testutil.ContextWithUser(context.Background(), requestUser, testDB.Queries())

		_, err := server.RequestItem(requestCtx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   requestUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)

		// Admin views all requests
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ViewAllData, nil, true, nil)
		adminCtx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		response, err := server.GetAllRequests(adminCtx, api.GetAllRequestsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetAllRequests200JSONResponse{}, response)

		requestsResp := response.(api.GetAllRequests200JSONResponse)
		assert.GreaterOrEqual(t, len(requestsResp), 1)
	})

	t.Run("member cannot view all requests", func(t *testing.T) {
		memberUser := testDB.NewUser(t).
			WithEmail("member@noviewall.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(memberUser.ID, rbac.ViewAllData, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), memberUser, testDB.Queries())

		response, err := server.GetAllRequests(ctx, api.GetAllRequestsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetAllRequests403JSONResponse{}, response)

		errorResp := response.(api.GetAllRequests403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
	})
}

func TestServer_GetPendingRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping get pending requests tests in short mode")
	}

	server, testDB, mockAuth := testBorrowingServer(t)

	t.Run("approver views pending requests", func(t *testing.T) {
		approverUser := testDB.NewUser(t).
			WithEmail("approver@pending.ca").
			AsApprover().
			Create()

		requestUser := testDB.NewUser(t).
			WithEmail("requester@pending.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Pending Group").
			Create()

		// Assign requester to group
		testDB.AssignUserToGroup(t, requestUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("iPad Pro").
			WithType("high").
			WithStock(2).
			Create()

		// Create a pending request
		mockAuth.ExpectCheckPermission(requestUser.ID, rbac.RequestItems, &group.ID, true, nil)
		requestCtx := testutil.ContextWithUser(context.Background(), requestUser, testDB.Queries())

		_, err := server.RequestItem(requestCtx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   requestUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)

		// Approver views pending requests
		mockAuth.ExpectCheckPermission(approverUser.ID, rbac.ApproveAllRequests, nil, true, nil)
		approverCtx := testutil.ContextWithUser(context.Background(), approverUser, testDB.Queries())

		response, err := server.GetPendingRequests(approverCtx, api.GetPendingRequestsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetPendingRequests200JSONResponse{}, response)

		pendingResp := response.(api.GetPendingRequests200JSONResponse)
		assert.GreaterOrEqual(t, len(pendingResp), 1)

		// Verify all returned requests are pending
		for _, req := range pendingResp {
			assert.Equal(t, api.Pending, req.Status)
		}
	})

	t.Run("member cannot view pending requests", func(t *testing.T) {
		memberUser := testDB.NewUser(t).
			WithEmail("member@nopending.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(memberUser.ID, rbac.ApproveAllRequests, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), memberUser, testDB.Queries())

		response, err := server.GetPendingRequests(ctx, api.GetPendingRequestsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetPendingRequests403JSONResponse{}, response)

		errorResp := response.(api.GetPendingRequests403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
	})
}

func TestServer_GetRequestsByUserId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping get requests by user id tests in short mode")
	}

	server, testDB, mockAuth := testBorrowingServer(t)

	t.Run("user views their own requests", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("user@ownrequests.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Own Requests Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("Surface Pro").
			WithType("high").
			WithStock(3).
			Create()

		// Create requests
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		_, err := server.RequestItem(ctx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   testUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)

		// View own requests
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)

		response, err := server.GetRequestsByUserId(ctx, api.GetRequestsByUserIdRequestObject{
			UserId: testUser.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetRequestsByUserId200JSONResponse{}, response)

		requestsResp := response.(api.GetRequestsByUserId200JSONResponse)
		assert.GreaterOrEqual(t, len(requestsResp), 1)

		// Verify all returned requests belong to this user
		for _, req := range requestsResp {
			assert.Equal(t, testUser.ID, req.UserId)
		}
	})

	t.Run("user cannot view another user's requests", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("user@view.ca").
			AsMember().
			Create()

		otherUser := testDB.NewUser(t).
			WithEmail("other@view.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetRequestsByUserId(ctx, api.GetRequestsByUserIdRequestObject{
			UserId: otherUser.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetRequestsByUserId403JSONResponse{}, response)

		errorResp := response.(api.GetRequestsByUserId403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "other users")
	})
}

func TestServer_GetRequestById(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping get request by id tests in short mode")
	}

	server, testDB, mockAuth := testBorrowingServer(t)

	t.Run("user views their own request", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("user@ownrequest.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("Own Request Group").
			Create()

		// Assign user to group
		testDB.AssignUserToGroup(t, testUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("GoPro").
			WithType("high").
			WithStock(2).
			Create()

		// Create request
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.RequestItems, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		requestResp, err := server.RequestItem(ctx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   testUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)
		createdRequest := requestResp.(api.RequestItem201JSONResponse)

		// View request by ID
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewAllData, nil, false, nil)

		response, err := server.GetRequestById(ctx, api.GetRequestByIdRequestObject{
			RequestId: createdRequest.Id,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetRequestById200JSONResponse{}, response)

		requestByIdResp := response.(api.GetRequestById200JSONResponse)
		assert.Equal(t, createdRequest.Id, requestByIdResp.Id)
		assert.Equal(t, testUser.ID, requestByIdResp.UserId)
	})

	t.Run("admin can view any user's request", func(t *testing.T) {
		adminUser := testDB.NewUser(t).
			WithEmail("admin@viewany.ca").
			AsGlobalAdmin().
			Create()

		requestUser := testDB.NewUser(t).
			WithEmail("requester@viewany.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("View Any Group").
			Create()

		// Assign requester to group
		testDB.AssignUserToGroup(t, requestUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("Sony Camera").
			WithType("high").
			WithStock(1).
			Create()

		// Create request
		mockAuth.ExpectCheckPermission(requestUser.ID, rbac.RequestItems, &group.ID, true, nil)
		requestCtx := testutil.ContextWithUser(context.Background(), requestUser, testDB.Queries())

		requestResp, err := server.RequestItem(requestCtx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   requestUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)
		createdRequest := requestResp.(api.RequestItem201JSONResponse)

		// Admin views request
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ViewOwnData, nil, true, nil)
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ViewAllData, nil, true, nil)
		adminCtx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		response, err := server.GetRequestById(adminCtx, api.GetRequestByIdRequestObject{
			RequestId: createdRequest.Id,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetRequestById200JSONResponse{}, response)

		requestByIdResp := response.(api.GetRequestById200JSONResponse)
		assert.Equal(t, createdRequest.Id, requestByIdResp.Id)
		assert.Equal(t, requestUser.ID, requestByIdResp.UserId)
	})

	t.Run("user cannot view another user's request", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("user@noaccess.ca").
			AsMember().
			Create()

		requestUser := testDB.NewUser(t).
			WithEmail("requester@noaccess.ca").
			AsMember().
			Create()

		group := testDB.NewGroup(t).
			WithName("No Access Group").
			Create()

		// Assign requester to group
		testDB.AssignUserToGroup(t, requestUser.ID, group.ID, "member")

		highItem := testDB.NewItem(t).
			WithName("Lens").
			WithType("high").
			WithStock(1).
			Create()

		// Create request
		mockAuth.ExpectCheckPermission(requestUser.ID, rbac.RequestItems, &group.ID, true, nil)
		requestCtx := testutil.ContextWithUser(context.Background(), requestUser, testDB.Queries())

		requestResp, err := server.RequestItem(requestCtx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   requestUser.ID,
				GroupId:  group.ID,
				ItemId:   highItem.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)
		createdRequest := requestResp.(api.RequestItem201JSONResponse)

		// Different user tries to view
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)
		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewAllData, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetRequestById(ctx, api.GetRequestByIdRequestObject{
			RequestId: createdRequest.Id,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetRequestById403JSONResponse{}, response)

		errorResp := response.(api.GetRequestById403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "view this request")
	})

	t.Run("request not found returns 404", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("user@notfound.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetRequestById(ctx, api.GetRequestByIdRequestObject{
			RequestId: uuid.New(),
		})

		require.NoError(t, err)
		require.IsType(t, api.GetRequestById404JSONResponse{}, response)

		errorResp := response.(api.GetRequestById404JSONResponse)
		assert.Equal(t, int32(404), errorResp.Code)
		assert.Contains(t, errorResp.Message, "not found")
	})
}

func TestServer_ReviewRequest_BookingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, mockJWT, mockAuth)

	t.Run("success - approve HIGH item creates booking", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// test data
		user := testDB.NewUser(t).WithEmail("user@reviewbooking.test").AsMember().Create()
		approver := testDB.NewUser(t).WithEmail("approver@reviewbooking.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()

		// Add user to group
		testDB.AssignUserToGroup(t, user.ID, group.ID, "member")

		userCtx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())
		approverCtx := testutil.ContextWithUser(context.Background(), approver, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(userCtx)
		require.NotEmpty(t, timeSlots)
		timeSlotID := timeSlots[0].ID

		// Create availability (7 days in future)
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(approverCtx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create request via RequestItem endpoint
		mockAuth.ExpectCheckPermission(user.ID, rbac.RequestItems, &group.ID, true, nil)

		requestResp, err := server.RequestItem(userCtx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   user.ID,
				GroupId:  group.ID,
				ItemId:   item.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)
		createdRequest := requestResp.(api.RequestItem201JSONResponse)

		// Test: Approver approves with booking fields
		mockAuth.ExpectCheckPermission(approver.ID, rbac.ApproveAllRequests, nil, true, nil)

		pickupLoc := "Main Office Lobby"
		returnLoc := "Main Office Return Desk"

		response, err := server.ReviewRequest(approverCtx, api.ReviewRequestRequestObject{
			RequestId: createdRequest.Id,
			Body: &api.ReviewRequestJSONRequestBody{
				Status:         api.Approved,
				AvailabilityId: &availability.ID,
				PickupLocation: &pickupLoc,
				ReturnLocation: &returnLoc,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ReviewRequest200JSONResponse{}, response)

		resp := response.(api.ReviewRequest200JSONResponse)
		assert.Equal(t, api.Approved, resp.Status)

		// Verify booking was created by checking the request has a booking_id
		request, err := testDB.Queries().GetRequestById(approverCtx, createdRequest.Id)
		require.NoError(t, err)
		assert.NotNil(t, request.BookingID, "Request should have a booking_id")

		// Verify booking details
		booking, err := testDB.Queries().GetBookingByID(approverCtx, *request.BookingID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, *booking.RequesterID)
		assert.Equal(t, approver.ID, *booking.ManagerID)
		assert.Equal(t, item.ID, *booking.ItemID)
		assert.Equal(t, availability.ID, *booking.AvailabilityID)
		assert.Equal(t, pickupLoc, booking.PickUpLocation)
		assert.Equal(t, returnLoc, booking.ReturnLocation)
		assert.Equal(t, db.RequestStatusPendingConfirmation, booking.Status)

		// Verify pickup date calculation (availability.date + time_slot.start_time)
		timeSlot, err := testDB.Queries().GetTimeSlotByID(approverCtx, timeSlotID)
		require.NoError(t, err)

		expectedPickupTime := futureDate.Add(time.Duration(timeSlot.StartTime.Microseconds) * time.Microsecond)
		assert.True(t, booking.PickUpDate.Time.Equal(expectedPickupTime) || booking.PickUpDate.Time.Sub(expectedPickupTime) < time.Second,
			"Pickup date should match availability date + time slot start time")

		// Verify return date calculation (pickup + 7 days)
		expectedReturnTime := expectedPickupTime.Add(7 * 24 * time.Hour)
		assert.True(t, booking.ReturnDate.Time.Equal(expectedReturnTime) || booking.ReturnDate.Time.Sub(expectedReturnTime) < time.Second,
			"Return date should be 7 days after pickup")
	})

	t.Run("bad request - approve HIGH item missing availability_id", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		user := testDB.NewUser(t).WithEmail("user@reviewbooking.test").AsMember().Create()
		approver := testDB.NewUser(t).WithEmail("approver@reviewbooking.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()

		testDB.AssignUserToGroup(t, user.ID, group.ID, "member")

		userCtx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())
		approverCtx := testutil.ContextWithUser(context.Background(), approver, testDB.Queries())

		// Create request via RequestItem endpoint
		mockAuth.ExpectCheckPermission(user.ID, rbac.RequestItems, &group.ID, true, nil)

		requestResp, err := server.RequestItem(userCtx, api.RequestItemRequestObject{
			Body: &api.RequestItemJSONRequestBody{
				UserId:   user.ID,
				GroupId:  group.ID,
				ItemId:   item.ID,
				Quantity: 1,
			},
		})
		require.NoError(t, err)
		createdRequest := requestResp.(api.RequestItem201JSONResponse)

		// Approve without availability_id
		mockAuth.ExpectCheckPermission(approver.ID, rbac.ApproveAllRequests, nil, true, nil)

		pickupLoc := "Main Office"
		returnLoc := "Main Office"

		response, err := server.ReviewRequest(approverCtx, api.ReviewRequestRequestObject{
			RequestId: createdRequest.Id,
			Body: &api.ReviewRequestJSONRequestBody{
				Status:         api.Approved,
				PickupLocation: &pickupLoc,
				ReturnLocation: &returnLoc,
				// Missing AvailabilityId
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ReviewRequest400JSONResponse{}, response)

		resp := response.(api.ReviewRequest400JSONResponse)
		assert.Equal(t, int32(400), resp.Code)
		assert.Contains(t, resp.Message, "availability_id")
	})

}
