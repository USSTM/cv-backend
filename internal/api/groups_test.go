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

func TestServer_GetAllGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, mockAuth := newTestServer(t)

	t.Run("successful get all groups", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("get@groups.ca").
			AsGlobalAdmin().
			Create()

		// Create some test groups
		testDB.NewGroup(t).
			WithName("USSTM").
			WithDescription("TMU Science Society").
			Create()

		testDB.NewGroup(t).
			WithName("PACS").
			WithDescription("Pratical Applications for Computer Science").
			Create()

		testDB.NewGroup(t).
			WithName("TMACC").
			WithDescription("TMU Algorithms Club").
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewGroupData, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetAllGroups(ctx, api.GetAllGroupsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetAllGroups200JSONResponse{}, response)

		groupsResp := response.(api.GetAllGroups200JSONResponse)
		assert.NotNil(t, groupsResp)
		assert.GreaterOrEqual(t, len(groupsResp), 3)
	})

	t.Run("unauthorized access (no permission)", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("noperm@groups.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewGroupData, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetAllGroups(ctx, api.GetAllGroupsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetAllGroups403JSONResponse{}, response)
	})
}

func TestServer_GetGroupByID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, mockAuth := newTestServer(t)

	t.Run("successful get group by id", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("get@groupsid.ca").
			AsGlobalAdmin().
			Create()

		group := testDB.NewGroup(t).
			WithName("Specific Group").
			WithDescription("Specific Description").
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewGroupData, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetGroupByID(ctx, api.GetGroupByIDRequestObject{
			Id: group.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetGroupByID200JSONResponse{}, response)

		groupResp := response.(api.GetGroupByID200JSONResponse)
		assert.Equal(t, group.ID, groupResp.Id)
		assert.Equal(t, group.Name, groupResp.Name)
		assert.Equal(t, group.Description, *groupResp.Description)
	})

	t.Run("group not found", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("notfound@groups.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewGroupData, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetGroupByID(ctx, api.GetGroupByIDRequestObject{
			Id: uuid.New(),
		})

		require.NoError(t, err)
		require.IsType(t, api.GetGroupByID404JSONResponse{}, response)
	})
}

func TestServer_CreateGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, mockAuth := newTestServer(t)

	t.Run("successful create group", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("create@groups.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageGroups, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		desc := "New Group Description"
		name := "New Created Group"

		response, err := server.CreateGroup(ctx, api.CreateGroupRequestObject{
			Body: &api.CreateGroupJSONRequestBody{
				Name:        name,
				Description: &desc,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CreateGroup201JSONResponse{}, response)

		groupResp := response.(api.CreateGroup201JSONResponse)
		assert.NotEqual(t, uuid.Nil, groupResp.Id)
		assert.Equal(t, name, groupResp.Name)
		assert.Equal(t, desc, *groupResp.Description)
	})

	t.Run("insufficient permissions", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("member@groups.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageGroups, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		desc := "Unauthorized Group"
		response, err := server.CreateGroup(ctx, api.CreateGroupRequestObject{
			Body: &api.CreateGroupJSONRequestBody{
				Name:        "Fail Group",
				Description: &desc,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CreateGroup403JSONResponse{}, response)
	})
}

func TestServer_UpdateGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, mockAuth := newTestServer(t)

	t.Run("successful update group", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("update@groups.ca").
			AsGlobalAdmin().
			Create()

		group := testDB.NewGroup(t).
			WithName("Old Name").
			WithDescription("Old Description").
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageGroups, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		newName := "Updated Name"
		newDesc := "Updated Description"

		response, err := server.UpdateGroup(ctx, api.UpdateGroupRequestObject{
			Id: group.ID,
			Body: &api.UpdateGroupJSONRequestBody{
				Name:        newName,
				Description: &newDesc,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.UpdateGroup200JSONResponse{}, response)

		groupResp := response.(api.UpdateGroup200JSONResponse)
		assert.Equal(t, group.ID, groupResp.Id)
		assert.Equal(t, newName, groupResp.Name)
		assert.Equal(t, newDesc, *groupResp.Description)
	})

	t.Run("update non-existent group", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("update404@groups.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageGroups, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		newName := "Ghost Group"
		desc := "Ghost Description"

		response, err := server.UpdateGroup(ctx, api.UpdateGroupRequestObject{
			Id: uuid.New(),
			Body: &api.UpdateGroupJSONRequestBody{
				Name:        newName,
				Description: &desc,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.UpdateGroup500JSONResponse{}, response)
	})
}

func TestServer_DeleteGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, mockAuth := newTestServer(t)

	t.Run("successful delete group", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("delete@groups.ca").
			AsGlobalAdmin().
			Create()

		group := testDB.NewGroup(t).
			WithName("Delete Me").
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageGroups, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.DeleteGroup(ctx, api.DeleteGroupRequestObject{
			Id: group.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.DeleteGroup204Response{}, response)

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ViewGroupData, nil, true, nil)
		getResp, err := server.GetGroupByID(ctx, api.GetGroupByIDRequestObject{Id: group.ID})
		require.NoError(t, err)
		require.IsType(t, api.GetGroupByID404JSONResponse{}, getResp)
	})
}
