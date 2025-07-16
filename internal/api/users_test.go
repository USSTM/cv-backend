package api

import (
	"context"
	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestServer_Users(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping users tests in short mode")
	}

	testDB := testutil.NewTestDatabase(t)
	testDB.RunMigrations(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)

	server := NewServer(testDB, mockJWT, mockAuth)

	t.Run("successful view users", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("view@users.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "manage_users", nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetUsers(ctx, api.GetUsersRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.GetUsers200JSONResponse{}, response)

		usersResp := response.(api.GetUsers200JSONResponse)
		assert.NotNil(t, usersResp)
	})

	t.Run("successful invite global admin as globalAdmin", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("inviteGlobal@globaladmin.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "manage_group_users", nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.InviteUser(ctx, api.InviteUserRequestObject{
			Body: &api.InviteUserJSONRequestBody{
				Email:    "global@globaladmin.ca",
				RoleName: "global_admin",
				Scope:    "global",
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.InviteUser201JSONResponse{}, response)

		inviteResp := response.(api.InviteUser201JSONResponse)
		assert.NotNil(t, inviteResp)
	})

	t.Run("successful invite user as group admin", func(t *testing.T) {
		group := testDB.
			NewGroup(t).
			WithName("Test Group").
			Create()

		testUser := testDB.NewUser(t).
			WithEmail("inviteMember@groupadmin.ca").
			AsGroupAdminOf(group).
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "manage_group_users", &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.InviteUser(ctx, api.InviteUserRequestObject{
			Body: &api.InviteUserJSONRequestBody{
				Email:    "member@member.ca",
				RoleName: "member",
				Scope:    "group",
				ScopeId:  &group.ID,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.InviteUser201JSONResponse{}, response)

		inviteResp := response.(api.InviteUser201JSONResponse)
		assert.NotNil(t, inviteResp)
	})

	t.Run("trying to invite user without right permissions", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("inviteUser@nopermission.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "manage_group_users", nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.InviteUser(ctx, api.InviteUserRequestObject{
			Body: &api.InviteUserJSONRequestBody{
				Email:    "none@none.ca",
				RoleName: "member",
				Scope:    "global",
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.InviteUser403JSONResponse{}, response)

		inviteResp := response.(api.InviteUser403JSONResponse)
		assert.Equal(t, int32(403), inviteResp.Code)
		assert.Equal(t, "Insufficient permissions", inviteResp.Message)
	})

	t.Run("trying to manage get users without right permissions", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("view@usersMember.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, "manage_users", nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetUsers(ctx, api.GetUsersRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.GetUsers403JSONResponse{}, response)

		usersResp := response.(api.GetUsers403JSONResponse)
		assert.Equal(t, int32(403), usersResp.Code)
		assert.Equal(t, "Insufficient permissions", usersResp.Message)
	})
}
