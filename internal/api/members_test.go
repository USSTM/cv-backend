package api

import (
	"context"
	"testing"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_GetCurrentMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, testDB, _, _ := newAuthTestServer(t)
	firstGroup := testDB.NewGroup(t).WithName("Alpha").Create()
	secondGroup := testDB.NewGroup(t).WithName("Beta").Create()
	member := testDB.NewUser(t).
		WithEmail("member-profile@example.com").
		AsMember().
		AsMemberOf(firstGroup).
		AsGroupAdminOf(firstGroup).
		AsMemberOf(secondGroup).
		Create()

	response, err := server.GetCurrentMember(testutil.ContextWithUser(context.Background(), member, testDB.Queries()), genapi.GetCurrentMemberRequestObject{})
	require.NoError(t, err)
	require.IsType(t, genapi.GetCurrentMember200JSONResponse{}, response)

	profile := genapi.CurrentMember(response.(genapi.GetCurrentMember200JSONResponse))
	assert.Equal(t, member.ID, profile.Id)
	assert.Equal(t, member.Email, string(profile.Email))
	assert.Len(t, profile.Roles, 4)
	require.Len(t, profile.Groups, 2)
	assert.Equal(t, firstGroup.ID, profile.Groups[0].Id)
	assert.Equal(t, "Alpha", profile.Groups[0].Name)
	assert.Equal(t, []string{"group_admin", "member"}, profile.Groups[0].Roles)
	assert.Equal(t, secondGroup.ID, profile.Groups[1].Id)
	assert.Equal(t, []string{"member"}, profile.Groups[1].Roles)
}

func TestServer_GetCurrentMemberUnauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	server, _, _, _ := newAuthTestServer(t)
	response, err := server.GetCurrentMember(context.Background(), genapi.GetCurrentMemberRequestObject{})
	require.NoError(t, err)
	require.IsType(t, genapi.GetCurrentMember401JSONResponse{}, response)
}
