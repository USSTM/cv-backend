package api

import (
	"context"
	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestServer_Items(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping items tests in short mode")
	}

	testDB := testutil.NewTestDatabase(t)
	testDB.RunMigrations(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)

	server := NewServer(testDB, mockJWT, mockAuth)

	t.Run("successful view items", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("view@items.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "view_items", nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetItems(ctx, api.GetItemsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetItems200JSONResponse{}, response)

		itemsResp := response.(api.GetItems200JSONResponse)
		assert.NotNil(t, itemsResp)
	})

	t.Run("successful view item by id", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("view@itemsid.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "view_items", nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		item := testDB.NewItem(t).
			WithName("Test Item").
			WithDescription("This is an item").
			WithType("low").
			WithStock(5).
			WithUrls([]string{"http://example.com/item1"}).
			Create()

		response, err := server.GetItemById(ctx, api.GetItemByIdRequestObject{
			Id: item.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetItemById200JSONResponse{}, response)

		itemResp := response.(api.GetItemById200JSONResponse)
		assert.Equal(t, item.ID, *itemResp.Id)
		assert.Equal(t, item.Name, *itemResp.Name)
		assert.Equal(t, item.Description, *itemResp.Description)
		assert.Equal(t, api.ItemType(item.Type), *itemResp.Type)
		assert.Equal(t, item.Stock, *itemResp.Stock)
		assert.Equal(t, item.Urls, *itemResp.Urls)
	})

	t.Run("successful view items by type", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("view@itemstype.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "view_items", nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		item := testDB.NewItem(t).
			WithName("Type Item").
			WithDescription("This is an item of type medium").
			WithType("medium").
			WithStock(10).
			WithUrls([]string{"http://example.com/typeitem"}).
			Create()

		response, err := server.GetItemsByType(ctx, api.GetItemsByTypeRequestObject{
			Type: api.ItemType(item.Type),
		})

		require.NoError(t, err)
		require.IsType(t, api.GetItemsByType200JSONResponse{}, response)

		itemsResp := response.(api.GetItemsByType200JSONResponse)
		assert.NotNil(t, itemsResp)
	})

	t.Run("successful create item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("create@items.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "manage_items", nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		desc := "This is a new item"

		response, err := server.CreateItem(ctx, api.CreateItemRequestObject{
			Body: &api.CreateItemJSONRequestBody{
				Name:        "New Item",
				Description: &desc,
				Type:        "low",
				Stock:       20,
				Urls:        &[]string{"http://example.com/newitem"},
			},
		})
		require.NoError(t, err)
		require.IsType(t, api.CreateItem201JSONResponse{}, response)

		itemResp := response.(api.CreateItem201JSONResponse)
		assert.NotNil(t, itemResp.Id)
		assert.Equal(t, "New Item", *itemResp.Name)
		assert.Equal(t, "This is a new item", *itemResp.Description)
		assert.Equal(t, api.ItemType("low"), *itemResp.Type)
		assert.Equal(t, 20, *itemResp.Stock)
		assert.Equal(t, []string{"http://example.com/newitem"}, *itemResp.Urls)
	})

	t.Run("successful update item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("update@items.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "manage_items", nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		item := testDB.NewItem(t).
			WithName("Old Item").
			WithDescription("Old description").
			WithType("low").
			WithStock(10).
			WithUrls([]string{"http://example.com/olditem"}).
			Create()

		newDesc := "Updated item description"

		response, err := server.UpdateItem(ctx, api.UpdateItemRequestObject{
			Id: item.ID,
			Body: &api.UpdateItemJSONRequestBody{
				Name:        "Updated Item",
				Description: &newDesc,
				Type:        "high",
				Stock:       25,
				Urls:        &[]string{"http://example.com/updateditem"},
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.UpdateItem200JSONResponse{}, response)

		itemResp := response.(api.UpdateItem200JSONResponse)
		assert.NotNil(t, itemResp.Id)
		assert.Equal(t, "Updated Item", *itemResp.Name)
		assert.Equal(t, "Updated item description", *itemResp.Description)
		assert.Equal(t, api.ItemType("high"), *itemResp.Type)
		assert.Equal(t, 25, *itemResp.Stock)
		assert.Equal(t, []string{"http://example.com/updateditem"}, *itemResp.Urls)
	})

	t.Run("successful delete item", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("delete@items.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "manage_items", nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		item := testDB.NewItem(t).
			WithName("Item to Delete").
			WithDescription("This item will be deleted").
			WithType("medium").
			WithStock(15).
			WithUrls([]string{"http://example.com/itemtodelete"}).
			Create()

		response, err := server.DeleteItem(ctx, api.DeleteItemRequestObject{
			Id: item.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.DeleteItem204Response{}, response)
	})

	t.Run("trying to find item that doesn't exist", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("findnonexistent@items.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "view_items", nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetItemById(ctx, api.GetItemByIdRequestObject{
			Id: uuid.Nil,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetItemById404JSONResponse{}, response)

		errorResp := response.(api.GetItemById404JSONResponse)
		assert.Equal(t, int32(404), errorResp.Code)
		assert.Equal(t, "Item not found", errorResp.Message)
	})

	t.Run("trying to create item without request body", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("create@itemsNoBody.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "manage_items", nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.CreateItem(ctx, api.CreateItemRequestObject{
			Body: nil,
		})

		require.NoError(t, err)
		require.IsType(t, api.CreateItem400JSONResponse{}, response)

		errorResp := response.(api.CreateItem400JSONResponse)
		assert.Equal(t, int32(400), errorResp.Code)
		assert.Equal(t, "Request body is required", errorResp.Message)
	})

	t.Run("trying to create item as a member (without permission)", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("create@itemsMember.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "manage_items", nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.CreateItem(ctx, api.CreateItemRequestObject{
			Body: &api.CreateItemJSONRequestBody{
				Name:        "Unauthorized Item",
				Description: nil,
				Type:        "low",
				Stock:       5,
				Urls:        nil,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CreateItem403JSONResponse{}, response)

		errorResp := response.(api.CreateItem403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Equal(t, "Insufficient permissions", errorResp.Message)
	})
}
