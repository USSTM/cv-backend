package api

import (
	"context"
	"testing"

	"github.com/USSTM/cv-backend/internal/rbac"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_Users(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping users tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)

	server := NewServer(testDB, testQueue, mockJWT, mockAuth)

	t.Run("successful view users", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("view@users.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageUsers, nil, true, nil)
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

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageGroupUsers, nil, true, nil)
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

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageGroupUsers, &group.ID, true, nil)
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

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageGroupUsers, nil, false, nil)
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

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageUsers, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetUsers(ctx, api.GetUsersRequestObject{})
		require.NoError(t, err)
		require.IsType(t, api.GetUsers403JSONResponse{}, response)

		usersResp := response.(api.GetUsers403JSONResponse)
		assert.Equal(t, int32(403), usersResp.Code)
		assert.Equal(t, "Insufficient permissions", usersResp.Message)
	})
}

func TestServer_GetUserById(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping get user by id tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)

	server := NewServer(testDB, testQueue, mockJWT, mockAuth)

	t.Run("successful get user by id as admin", func(t *testing.T) {
		adminUser := testDB.NewUser(t).
			WithEmail("admin@getuser.ca").
			AsGlobalAdmin().
			Create()

		targetUser := testDB.NewUser(t).
			WithEmail("target@user.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageUsers, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		response, err := server.GetUserById(ctx, api.GetUserByIdRequestObject{
			UserId: targetUser.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserById200JSONResponse{}, response)

		userResp := response.(api.GetUserById200JSONResponse)
		assert.Equal(t, targetUser.ID, userResp.Id)
		assert.Equal(t, targetUser.Email, string(userResp.Email))
		assert.Equal(t, api.Member, userResp.Role)
	})

	t.Run("successful get own user by id", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("self@user.ca").
			AsMember().
			Create()

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetUserById(ctx, api.GetUserByIdRequestObject{
			UserId: testUser.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserById200JSONResponse{}, response)

		userResp := response.(api.GetUserById200JSONResponse)
		assert.Equal(t, testUser.ID, userResp.Id)
		assert.Equal(t, testUser.Email, string(userResp.Email))
		assert.Equal(t, api.Member, userResp.Role)
	})

	t.Run("unauthorized access to other user", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("unauthorized@user.ca").
			AsMember().
			Create()

		otherUser := testDB.NewUser(t).
			WithEmail("other@user.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageUsers, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetUserById(ctx, api.GetUserByIdRequestObject{
			UserId: otherUser.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserById403JSONResponse{}, response)

		errorResp := response.(api.GetUserById403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Equal(t, "Insufficient permissions", errorResp.Message)
	})

	t.Run("user not found", func(t *testing.T) {
		adminUser := testDB.NewUser(t).
			WithEmail("admin@notfound.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageUsers, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		nonExistentID := uuid.New()

		response, err := server.GetUserById(ctx, api.GetUserByIdRequestObject{
			UserId: nonExistentID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserById404JSONResponse{}, response)

		errorResp := response.(api.GetUserById404JSONResponse)
		assert.Equal(t, int32(404), errorResp.Code)
		assert.Equal(t, "User not found", errorResp.Message)
	})
}

func TestServer_GetUserByEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping get user by email tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)

	server := NewServer(testDB, testQueue, mockJWT, mockAuth)

	t.Run("successful get user by email as admin", func(t *testing.T) {
		adminUser := testDB.NewUser(t).
			WithEmail("admin@getuserbyemail.ca").
			AsGlobalAdmin().
			Create()

		targetUser := testDB.NewUser(t).
			WithEmail("target@email.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageUsers, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		response, err := server.GetUserByEmail(ctx, api.GetUserByEmailRequestObject{
			Email: types.Email(targetUser.Email),
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserByEmail200JSONResponse{}, response)

		userResp := response.(api.GetUserByEmail200JSONResponse)
		assert.Equal(t, targetUser.ID, userResp.Id)
		assert.Equal(t, targetUser.Email, string(userResp.Email))
		assert.Equal(t, api.Member, userResp.Role)
	})

	t.Run("unauthorized access by member", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("member@unauthorized.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageUsers, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetUserByEmail(ctx, api.GetUserByEmailRequestObject{
			Email: "someone@else.ca",
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserByEmail403JSONResponse{}, response)

		errorResp := response.(api.GetUserByEmail403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Equal(t, "Insufficient permissions", errorResp.Message)
	})

	t.Run("user not found by email", func(t *testing.T) {
		adminUser := testDB.NewUser(t).
			WithEmail("admin@emailnotfound.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageUsers, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		response, err := server.GetUserByEmail(ctx, api.GetUserByEmailRequestObject{
			Email: "nonexistent@user.ca",
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserByEmail404JSONResponse{}, response)

		errorResp := response.(api.GetUserByEmail404JSONResponse)
		assert.Equal(t, int32(404), errorResp.Code)
		assert.Equal(t, "User not found", errorResp.Message)
	})
}

func TestServer_GetUsersByGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping get users by group tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)

	server := NewServer(testDB, testQueue, mockJWT, mockAuth)

	t.Run("successful get users by group", func(t *testing.T) {
		group := testDB.NewGroup(t).
			WithName("Test Group").
			WithDescription("A group for testing").
			Create()

		adminUser := testDB.NewUser(t).
			WithEmail("admin@getusersgroup.ca").
			AsGlobalAdmin().
			Create()

		groupUser1 := testDB.NewUser(t).
			WithEmail("member1@group.ca").
			AsMemberOf(group).
			Create()

		groupUser2 := testDB.NewUser(t).
			WithEmail("admin@group.ca").
			AsGroupAdminOf(group).
			Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageGroupUsers, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		response, err := server.GetUsersByGroup(ctx, api.GetUsersByGroupRequestObject{
			GroupId: group.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUsersByGroup200JSONResponse{}, response)

		usersResp := response.(api.GetUsersByGroup200JSONResponse)
		assert.NotNil(t, usersResp)
		assert.Len(t, usersResp, 2)

		userEmails := make([]string, len(usersResp))
		for i, user := range usersResp {
			userEmails[i] = string(user.Email)
		}
		assert.Contains(t, userEmails, groupUser1.Email)
		assert.Contains(t, userEmails, groupUser2.Email)
	})

	t.Run("unauthorized access to group users", func(t *testing.T) {
		group := testDB.NewGroup(t).
			WithName("Private Group").
			WithDescription("A private group").
			Create()

		testUser := testDB.NewUser(t).
			WithEmail("unauthorized@groupusers.ca").
			AsMember().
			Create()

		mockAuth.ExpectCheckPermission(testUser.ID, rbac.ManageGroupUsers, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.GetUsersByGroup(ctx, api.GetUsersByGroupRequestObject{
			GroupId: group.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUsersByGroup403JSONResponse{}, response)

		errorResp := response.(api.GetUsersByGroup403JSONResponse)
		assert.Equal(t, int32(403), errorResp.Code)
		assert.Equal(t, "Insufficient permissions", errorResp.Message)
	})

	t.Run("group not found", func(t *testing.T) {
		adminUser := testDB.NewUser(t).
			WithEmail("admin@groupnotfound.ca").
			AsGlobalAdmin().
			Create()

		nonExistentGroupID := uuid.New()
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageGroupUsers, &nonExistentGroupID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		response, err := server.GetUsersByGroup(ctx, api.GetUsersByGroupRequestObject{
			GroupId: nonExistentGroupID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUsersByGroup404JSONResponse{}, response)

		errorResp := response.(api.GetUsersByGroup404JSONResponse)
		assert.Equal(t, int32(404), errorResp.Code)
		assert.Equal(t, "Group not found", errorResp.Message)
	})

	t.Run("no users found in group", func(t *testing.T) {
		emptyGroup := testDB.NewGroup(t).
			WithName("Empty Group").
			WithDescription("A group with no users").
			Create()

		adminUser := testDB.NewUser(t).
			WithEmail("admin@emptygroup.ca").
			AsGlobalAdmin().
			Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageGroupUsers, &emptyGroup.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		response, err := server.GetUsersByGroup(ctx, api.GetUsersByGroupRequestObject{
			GroupId: emptyGroup.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUsersByGroup404JSONResponse{}, response)

		errorResp := response.(api.GetUsersByGroup404JSONResponse)
		assert.Equal(t, int32(404), errorResp.Code)
		assert.Equal(t, "No users found in the specified group", errorResp.Message)
	})
}
