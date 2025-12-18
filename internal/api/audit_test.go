package api

import (
	"github.com/USSTM/cv-backend/internal/rbac"
	"context"
	"testing"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAuditServer(t *testing.T) (*Server, *testutil.TestDatabase, *testutil.MockAuthenticator) {
	testDB := getSharedTestDatabase(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)

	server := NewServer(testDB, mockJWT, mockAuth)

	return server, testDB, mockAuth
}

// Helper to create taking record
func createTaking(t *testing.T, testDB *testutil.TestDatabase, userID, groupID, itemID uuid.UUID, quantity int32) uuid.UUID {
	taking, err := testDB.Queries().RecordItemTaking(context.Background(), db.RecordItemTakingParams{
		UserID:   userID,
		GroupID:  groupID,
		ItemID:   itemID,
		Quantity: quantity,
	})
	require.NoError(t, err)
	return taking.ID
}

func TestServer_GetUserTakingHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping audit tests in short mode")
	}

	server, testDB, mockAuth := testAuditServer(t)

	t.Run("user can view their own taking history", func(t *testing.T) {
		// Create user
		testUser := testDB.NewUser(t).
			WithEmail("user@own.com").
			AsMember().
			Create()

		// Create group and item
		group := testDB.NewGroup(t).
			WithName("User Group").
			Create()

		item := testDB.NewItem(t).
			WithName("USB Cable").
			WithType("low").
			WithStock(100).
			Create()

		// Create taking record
		createTaking(t, testDB, testUser.ID, group.ID, item.ID, 2)
		createTaking(t, testDB, testUser.ID, group.ID, item.ID, 3)

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		// Call handler
		response, err := server.GetUserTakingHistory(ctx, api.GetUserTakingHistoryRequestObject{
			UserId: testUser.ID,
			Params: api.GetUserTakingHistoryParams{},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserTakingHistory200JSONResponse{}, response)

		takingResp := response.(api.GetUserTakingHistory200JSONResponse)
		assert.Len(t, takingResp, 2, "User should see their own 2 taking records")
		assert.Equal(t, testUser.ID, takingResp[0].UserId)
		assert.Equal(t, "USB Cable", takingResp[0].ItemName)
	})

	t.Run("user cannot view another user's taking history", func(t *testing.T) {
		// Create users
		requestingUser := testDB.NewUser(t).
			WithEmail("requester@test.com").
			AsMember().
			Create()

		targetUser := testDB.NewUser(t).
			WithEmail("target@test.com").
			AsMember().
			Create()

		// Create group and item
		group := testDB.NewGroup(t).
			WithName("Test Group").
			Create()

		item := testDB.NewItem(t).
			WithName("Battery").
			WithType("low").
			WithStock(50).
			Create()

		// Create taking record for target user
		createTaking(t, testDB, targetUser.ID, group.ID, item.ID, 5)

		// requesting user does NOT have view_all_data
		mockAuth.ExpectCheckPermission(requestingUser.ID, rbac.ViewOwnData, nil, true, nil)
		mockAuth.ExpectCheckPermission(requestingUser.ID, rbac.ViewAllData, nil, false, nil)

		ctx := testutil.ContextWithUser(context.Background(), requestingUser, testDB.Queries())

		// view target user's history
		response, err := server.GetUserTakingHistory(ctx, api.GetUserTakingHistoryRequestObject{
			UserId: targetUser.ID,
			Params: api.GetUserTakingHistoryParams{},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserTakingHistory403JSONResponse{}, response)

		errorResp := response.(api.GetUserTakingHistory403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient permissions")
	})

	t.Run("global admin can view any user's taking history", func(t *testing.T) {
		// Create admin and user
		admin := testDB.NewUser(t).
			WithEmail("admin@global.com").
			AsGlobalAdmin().
			Create()

		targetUser := testDB.NewUser(t).
			WithEmail("user@test.com").
			AsMember().
			Create()

		// Create group and item
		group := testDB.NewGroup(t).
			WithName("Admin Group").
			Create()

		item := testDB.NewItem(t).
			WithName("HDMI Cable").
			WithType("low").
			WithStock(75).
			Create()

		// Create taking record for target user
		createTaking(t, testDB, targetUser.ID, group.ID, item.ID, 1)
		createTaking(t, testDB, targetUser.ID, group.ID, item.ID, 2)

		// admin has view_all_data
		mockAuth.ExpectCheckPermission(admin.ID, rbac.ViewOwnData, nil, true, nil)
		mockAuth.ExpectCheckPermission(admin.ID, rbac.ViewAllData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		// admin views user's history
		response, err := server.GetUserTakingHistory(ctx, api.GetUserTakingHistoryRequestObject{
			UserId: targetUser.ID,
			Params: api.GetUserTakingHistoryParams{},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserTakingHistory200JSONResponse{}, response)

		takingResp := response.(api.GetUserTakingHistory200JSONResponse)
		assert.Len(t, takingResp, 2, "Admin should see all 2 taking records")
		assert.Equal(t, targetUser.ID, takingResp[0].UserId)
	})

	t.Run("group admin can view member's history with valid groupId filter", func(t *testing.T) {
		// Create group admin and member
		groupAdmin := testDB.NewUser(t).
			WithEmail("groupadmin@test.com").
			Create()

		memberUser := testDB.NewUser(t).
			WithEmail("member@test.com").
			Create()

		// Create group
		group := testDB.NewGroup(t).
			WithName("Engineering Group").
			Create()

		// Assign roles
		testDB.AssignUserToGroup(t, groupAdmin.ID, group.ID, "group_admin")
		testDB.AssignUserToGroup(t, memberUser.ID, group.ID, "member")

		// Create item
		item := testDB.NewItem(t).
			WithName("Arduino Kit").
			WithType("low").
			WithStock(20).
			Create()

		// Create taking record for member
		createTaking(t, testDB, memberUser.ID, group.ID, item.ID, 1)

		mockAuth.ExpectCheckPermission(groupAdmin.ID, rbac.ViewOwnData, nil, true, nil)
		mockAuth.ExpectCheckPermission(groupAdmin.ID, rbac.ViewAllData, nil, false, nil)
		mockAuth.ExpectCheckPermission(groupAdmin.ID, rbac.ViewGroupData, &group.ID, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), groupAdmin, testDB.Queries())

		// Group admin views history with groupId filter
		response, err := server.GetUserTakingHistory(ctx, api.GetUserTakingHistoryRequestObject{
			UserId: memberUser.ID,
			Params: api.GetUserTakingHistoryParams{
				GroupId: &group.ID,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserTakingHistory200JSONResponse{}, response)

		takingResp := response.(api.GetUserTakingHistory200JSONResponse)
		assert.Len(t, takingResp, 1, "Group admin should see member's taking record")
		assert.Equal(t, memberUser.ID, takingResp[0].UserId)
		assert.Equal(t, group.ID, takingResp[0].GroupId)
	})

	t.Run("group admin must specify groupId to view other users", func(t *testing.T) {
		// Create group admin and member
		groupAdmin := testDB.NewUser(t).
			WithEmail("admin@groupA.com").
			Create()

		memberUser := testDB.NewUser(t).
			WithEmail("user@groupA.com").
			Create()

		// Create group
		groupA := testDB.NewGroup(t).
			WithName("Group A").
			Create()

		// Both users in same group
		testDB.AssignUserToGroup(t, groupAdmin.ID, groupA.ID, "group_admin")
		testDB.AssignUserToGroup(t, memberUser.ID, groupA.ID, "member")

		// Create item
		item := testDB.NewItem(t).
			WithName("Power Supply").
			WithType("low").
			WithStock(15).
			Create()

		// Create taking record for member
		createTaking(t, testDB, memberUser.ID, groupA.ID, item.ID, 1)

		// group admin tries WITHOUT filter
		mockAuth.ExpectCheckPermission(groupAdmin.ID, rbac.ViewOwnData, nil, true, nil)
		mockAuth.ExpectCheckPermission(groupAdmin.ID, rbac.ViewAllData, nil, false, nil)

		ctx := testutil.ContextWithUser(context.Background(), groupAdmin, testDB.Queries())

		response, err := server.GetUserTakingHistory(ctx, api.GetUserTakingHistoryRequestObject{
			UserId: memberUser.ID,
			Params: api.GetUserTakingHistoryParams{},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserTakingHistory403JSONResponse{}, response)

		errorResp := response.(api.GetUserTakingHistory403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient permissions")
	})

	t.Run("group admin cannot access another group's data", func(t *testing.T) {
		// Create group admin and member
		groupAdmin := testDB.NewUser(t).
			WithEmail("admin@groupX.com").
			Create()

		memberUser := testDB.NewUser(t).
			WithEmail("member@groupY.com").
			Create()

		// Create two groups
		groupX := testDB.NewGroup(t).
			WithName("Group X").
			Create()

		groupY := testDB.NewGroup(t).
			WithName("Group Y").
			Create()

		// groupAdmin is admin of Group X, member is in Group Y
		testDB.AssignUserToGroup(t, groupAdmin.ID, groupX.ID, "group_admin")
		testDB.AssignUserToGroup(t, memberUser.ID, groupY.ID, "member")

		// Create item
		item := testDB.NewItem(t).
			WithName("Raspberry Pi").
			WithType("low").
			WithStock(10).
			Create()

		// Create taking record for member
		createTaking(t, testDB, memberUser.ID, groupY.ID, item.ID, 1)

		// admin tries to access Group Y (not their group)
		mockAuth.ExpectCheckPermission(groupAdmin.ID, rbac.ViewOwnData, nil, true, nil)
		mockAuth.ExpectCheckPermission(groupAdmin.ID, rbac.ViewAllData, nil, false, nil)
		mockAuth.ExpectCheckPermission(groupAdmin.ID, rbac.ViewGroupData, &groupY.ID, false, nil)

		ctx := testutil.ContextWithUser(context.Background(), groupAdmin, testDB.Queries())

		response, err := server.GetUserTakingHistory(ctx, api.GetUserTakingHistoryRequestObject{
			UserId: memberUser.ID,
			Params: api.GetUserTakingHistoryParams{
				GroupId: &groupY.ID,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserTakingHistory403JSONResponse{}, response)

		errorResp := response.(api.GetUserTakingHistory403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
	})

	t.Run("pagination works correctly", func(t *testing.T) {
		// Create user
		testUser := testDB.NewUser(t).
			WithEmail("paginate@test.com").
			AsMember().
			Create()

		// Create group and item
		group := testDB.NewGroup(t).
			WithName("Pagination Group").
			Create()

		item := testDB.NewItem(t).
			WithName("Test Item").
			WithType("low").
			WithStock(1000).
			Create()

		// Create 10 taking record
		for i := range 10 {
			createTaking(t, testDB, testUser.ID, group.ID, item.ID, int32(i+1))
		}

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		// get first 5
		limit := 5
		offset := 0
		response, err := server.GetUserTakingHistory(ctx, api.GetUserTakingHistoryRequestObject{
			UserId: testUser.ID,
			Params: api.GetUserTakingHistoryParams{
				Limit:  &limit,
				Offset: &offset,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserTakingHistory200JSONResponse{}, response)

		takingResp := response.(api.GetUserTakingHistory200JSONResponse)
		assert.Len(t, takingResp, 5, "Should return only 5 records with limit=5")
	})

	t.Run("empty history returns empty array not null", func(t *testing.T) {
		// Create user with no taking history
		testUser := testDB.NewUser(t).
			WithEmail("empty@test.com").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewOwnData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetUserTakingHistory(ctx, api.GetUserTakingHistoryRequestObject{
			UserId: testUser.ID,
			Params: api.GetUserTakingHistoryParams{},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserTakingHistory200JSONResponse{}, response)

		takingResp := response.(api.GetUserTakingHistory200JSONResponse)
		assert.NotNil(t, takingResp, "Response should not be nil")
		assert.Len(t, takingResp, 0, "Response should be empty array")
	})
}

func TestServer_GetItemTakingHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping audit tests in short mode")
	}

	server, testDB, mockAuth := testAuditServer(t)

	t.Run("admin can view item taking history", func(t *testing.T) {
		// Create admin
		admin := testDB.NewUser(t).
			WithEmail("admin@takinghistory.com").
			AsGlobalAdmin().
			Create()

		// Create group and item
		group := testDB.NewGroup(t).
			WithName("Test Group").
			Create()

		item := testDB.NewItem(t).
			WithName("USB Cable").
			WithType("low").
			WithStock(100).
			Create()

		// Create multiple users and records
		user1 := testDB.NewUser(t).
			WithEmail("user1@test.com").
			AsMember().
			Create()

		user2 := testDB.NewUser(t).
			WithEmail("user2@test.com").
			AsMember().
			Create()

		createTaking(t, testDB, user1.ID, group.ID, item.ID, 2)
		createTaking(t, testDB, user2.ID, group.ID, item.ID, 3)
		createTaking(t, testDB, user1.ID, group.ID, item.ID, 1)

		mockAuth.ExpectCheckPermission(admin.ID, rbac.ViewAllData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		// Call handler
		response, err := server.GetItemTakingHistory(ctx, api.GetItemTakingHistoryRequestObject{
			ItemId: item.ID,
			Params: api.GetItemTakingHistoryParams{},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetItemTakingHistory200JSONResponse{}, response)

		historyResp := response.(api.GetItemTakingHistory200JSONResponse)
		assert.Len(t, historyResp, 3, "Should return all 3 taking records for the item")
		assert.Equal(t, item.ID, historyResp[0].ItemId)
	})

	t.Run("non-admin cannot view item taking history", func(t *testing.T) {
		// Create user
		regularUser := testDB.NewUser(t).
			WithEmail("regular@user.com").
			AsMember().
			Create()

		// Create group and item
		group := testDB.NewGroup(t).
			WithName("Regular Group").
			Create()

		item := testDB.NewItem(t).
			WithName("Battery").
			WithType("low").
			WithStock(50).
			Create()

		// Create taking record
		createTaking(t, testDB, regularUser.ID, group.ID, item.ID, 5)

		// user does NOT have view_all_data
		mockAuth.ExpectCheckPermission(regularUser.ID, rbac.ViewAllData, nil, false, nil)

		ctx := testutil.ContextWithUser(context.Background(), regularUser, testDB.Queries())

		// view item history
		response, err := server.GetItemTakingHistory(ctx, api.GetItemTakingHistoryRequestObject{
			ItemId: item.ID,
			Params: api.GetItemTakingHistoryParams{},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetItemTakingHistory403JSONResponse{}, response)

		errorResp := response.(api.GetItemTakingHistory403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient permissions")
	})

	t.Run("pagination works correctly for item history", func(t *testing.T) {
		// Create admin
		admin := testDB.NewUser(t).
			WithEmail("admin@pagination.com").
			AsGlobalAdmin().
			Create()

		// Create group and item
		group := testDB.NewGroup(t).
			WithName("Pagination Group").
			Create()

		item := testDB.NewItem(t).
			WithName("HDMI Cable").
			WithType("low").
			WithStock(200).
			Create()

		// Create user
		user := testDB.NewUser(t).
			WithEmail("user@pagination.com").
			AsMember().
			Create()

		for i := range 10 {
			createTaking(t, testDB, user.ID, group.ID, item.ID, int32(i+1))
		}

		mockAuth.ExpectCheckPermission(admin.ID, rbac.ViewAllData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		// get first 5
		limit := 5
		offset := 0
		response, err := server.GetItemTakingHistory(ctx, api.GetItemTakingHistoryRequestObject{
			ItemId: item.ID,
			Params: api.GetItemTakingHistoryParams{
				Limit:  &limit,
				Offset: &offset,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetItemTakingHistory200JSONResponse{}, response)

		historyResp := response.(api.GetItemTakingHistory200JSONResponse)
		assert.Len(t, historyResp, 5, "Should return only 5 records with limit=5")
	})

	t.Run("empty item history returns empty array not null", func(t *testing.T) {
		// Create admin
		admin := testDB.NewUser(t).
			WithEmail("admin@empty.com").
			AsGlobalAdmin().
			Create()

		// Create item with no history
		item := testDB.NewItem(t).
			WithName("Unused Item").
			WithType("low").
			WithStock(10).
			Create()

		mockAuth.ExpectCheckPermission(admin.ID, rbac.ViewAllData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		response, err := server.GetItemTakingHistory(ctx, api.GetItemTakingHistoryRequestObject{
			ItemId: item.ID,
			Params: api.GetItemTakingHistoryParams{},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetItemTakingHistory200JSONResponse{}, response)

		historyResp := response.(api.GetItemTakingHistory200JSONResponse)
		assert.NotNil(t, historyResp, "Response should not be nil")
		assert.Len(t, historyResp, 0, "Response should be empty array")
	})
}

func TestServer_GetItemTakingStats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping audit tests in short mode")
	}

	server, testDB, mockAuth := testAuditServer(t)

	t.Run("admin can view item taking stats", func(t *testing.T) {
		// Create admin
		admin := testDB.NewUser(t).
			WithEmail("admin@stats.com").
			AsGlobalAdmin().
			Create()

		// Create group and item
		group := testDB.NewGroup(t).
			WithName("Stats Group").
			Create()

		item := testDB.NewItem(t).
			WithName("Power Adapter").
			WithType("low").
			WithStock(50).
			Create()

		// Create users
		user1 := testDB.NewUser(t).
			WithEmail("user1@stats.com").
			AsMember().
			Create()

		user2 := testDB.NewUser(t).
			WithEmail("user2@stats.com").
			AsMember().
			Create()

		// Create taking records
		createTaking(t, testDB, user1.ID, group.ID, item.ID, 5)
		createTaking(t, testDB, user2.ID, group.ID, item.ID, 3)
		createTaking(t, testDB, user1.ID, group.ID, item.ID, 2)

		mockAuth.ExpectCheckPermission(admin.ID, rbac.ViewAllData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		// Call handler without date
		response, err := server.GetItemTakingStats(ctx, api.GetItemTakingStatsRequestObject{
			ItemId: item.ID,
			Params: api.GetItemTakingStatsParams{},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetItemTakingStats200JSONResponse{}, response)

		statsResp := response.(api.GetItemTakingStats200JSONResponse)
		assert.Equal(t, 3, statsResp.TotalTakings, "Should have 3 total takings")
		assert.Equal(t, 10, statsResp.TotalQuantity, "Total quantity should be 5+3+2=10")
		assert.Equal(t, 2, statsResp.UniqueUsers, "Should have 2 unique users")
		assert.NotNil(t, statsResp.FirstTaking, "FirstTaking should not be nil")
		assert.NotNil(t, statsResp.LastTaking, "LastTaking should not be nil")
	})

	t.Run("non-admin cannot view item taking stats", func(t *testing.T) {
		// Create user
		regularUser := testDB.NewUser(t).
			WithEmail("regular@stats.com").
			AsMember().
			Create()

		// Create group and item
		group := testDB.NewGroup(t).
			WithName("Regular Stats Group").
			Create()

		item := testDB.NewItem(t).
			WithName("Ethernet Cable").
			WithType("low").
			WithStock(30).
			Create()

		// Create taking record
		createTaking(t, testDB, regularUser.ID, group.ID, item.ID, 2)

		// user does NOT have view_all_data
		mockAuth.ExpectCheckPermission(regularUser.ID, rbac.ViewAllData, nil, false, nil)

		ctx := testutil.ContextWithUser(context.Background(), regularUser, testDB.Queries())

		response, err := server.GetItemTakingStats(ctx, api.GetItemTakingStatsRequestObject{
			ItemId: item.ID,
			Params: api.GetItemTakingStatsParams{},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetItemTakingStats403JSONResponse{}, response)

		errorResp := response.(api.GetItemTakingStats403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Contains(t, errorResp.Message, "Insufficient permissions")
	})

	t.Run("item with no takings returns zero stats", func(t *testing.T) {
		// Create admin
		admin := testDB.NewUser(t).
			WithEmail("admin@zerostats.com").
			AsGlobalAdmin().
			Create()

		// Create item with no history
		item := testDB.NewItem(t).
			WithName("Brand New Item").
			WithType("low").
			WithStock(15).
			Create()

		mockAuth.ExpectCheckPermission(admin.ID, rbac.ViewAllData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		response, err := server.GetItemTakingStats(ctx, api.GetItemTakingStatsRequestObject{
			ItemId: item.ID,
			Params: api.GetItemTakingStatsParams{},
		})

		require.NoError(t, err)
		if errResp, ok := response.(api.GetItemTakingStats500JSONResponse); ok {
			t.Fatalf("Got 500 error: %s", errResp.Message)
		}
		require.IsType(t, api.GetItemTakingStats200JSONResponse{}, response)

		statsResp := response.(api.GetItemTakingStats200JSONResponse)
		assert.Equal(t, 0, statsResp.TotalTakings, "Should have 0 total takings")
		assert.Equal(t, 0, statsResp.TotalQuantity, "Total quantity should be 0")
		assert.Equal(t, 0, statsResp.UniqueUsers, "Should have 0 unique users")
		assert.Nil(t, statsResp.FirstTaking, "FirstTaking should be nil for zero stats")
		assert.Nil(t, statsResp.LastTaking, "LastTaking should be nil for zero stats")
	})

	t.Run("stats respect date range filters", func(t *testing.T) {
		// Create admin
		admin := testDB.NewUser(t).
			WithEmail("admin@datefilter.com").
			AsGlobalAdmin().
			Create()

		// Create group and item
		group := testDB.NewGroup(t).
			WithName("Date Filter Group").
			Create()

		item := testDB.NewItem(t).
			WithName("Filtered Item").
			WithType("low").
			WithStock(100).
			Create()

		// Create user
		user := testDB.NewUser(t).
			WithEmail("user@datefilter.com").
			AsMember().
			Create()

		// Create multiple records (similar timestamps)
		createTaking(t, testDB, user.ID, group.ID, item.ID, 10)
		createTaking(t, testDB, user.ID, group.ID, item.ID, 5)

		mockAuth.ExpectCheckPermission(admin.ID, rbac.ViewAllData, nil, true, nil)

		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		// no date filters should return all stats
		response, err := server.GetItemTakingStats(ctx, api.GetItemTakingStatsRequestObject{
			ItemId: item.ID,
			Params: api.GetItemTakingStatsParams{},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetItemTakingStats200JSONResponse{}, response)

		statsResp := response.(api.GetItemTakingStats200JSONResponse)
		assert.Equal(t, 2, statsResp.TotalTakings, "Should have 2 takings without date filter")
		assert.Equal(t, 15, statsResp.TotalQuantity, "Total quantity should be 10+5=15")
	})
}
