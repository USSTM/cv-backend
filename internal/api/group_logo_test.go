package api

import (
	"context"
	"testing"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadGroupLogo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("success with square image for group admin", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		group := testDB.NewGroup(t).WithName("LogoGroup").Create()
		admin := testDB.NewUser(t).WithEmail("logo@admin.ca").AsGroupAdminOf(group).Create()

		mockAuth.ExpectCheckPermission(admin.ID, rbac.ManageGroupUsers, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		reader := createJPEGMultipartReader(t, 300, 300, nil)
		resp, err := server.UploadGroupLogo(ctx, genapi.UploadGroupLogoRequestObject{
			GroupId: group.ID,
			Body:    reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadGroupLogo200JSONResponse{}, resp)

		logoResp := resp.(genapi.UploadGroupLogo200JSONResponse)
		assert.Equal(t, group.ID, logoResp.Id)
		assert.NotNil(t, logoResp.LogoUrl)
		assert.NotNil(t, logoResp.LogoThumbnailUrl)
		assert.NotEmpty(t, *logoResp.LogoUrl)
		assert.NotEmpty(t, *logoResp.LogoThumbnailUrl)
	})

	t.Run("rejects non-square image with 400", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		group := testDB.NewGroup(t).WithName("NSLogoGroup").Create()
		admin := testDB.NewUser(t).WithEmail("nslogo@admin.ca").AsGroupAdminOf(group).Create()

		mockAuth.ExpectCheckPermission(admin.ID, rbac.ManageGroupUsers, &group.ID, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		reader := createJPEGMultipartReader(t, 400, 200, nil) // non-square
		resp, err := server.UploadGroupLogo(ctx, genapi.UploadGroupLogoRequestObject{
			GroupId: group.ID,
			Body:    reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadGroupLogo400JSONResponse{}, resp)
	})

	t.Run("member without manage_group_users gets 403", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		group := testDB.NewGroup(t).WithName("DenyLogoGroup").Create()
		member := testDB.NewUser(t).WithEmail("member@logo.ca").AsMemberOf(group).Create()

		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageGroupUsers, &group.ID, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		reader := createJPEGMultipartReader(t, 300, 300, nil)
		resp, err := server.UploadGroupLogo(ctx, genapi.UploadGroupLogoRequestObject{
			GroupId: group.ID,
			Body:    reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadGroupLogo403JSONResponse{}, resp)
	})

	t.Run("old logo S3 keys replaced when new logo uploaded", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		group := testDB.NewGroup(t).WithName("ReplaceLogoGroup").Create()
		admin := testDB.NewUser(t).WithEmail("replace@logo.ca").AsGroupAdminOf(group).Create()
		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		// first logo
		mockAuth.ExpectCheckPermission(admin.ID, rbac.ManageGroupUsers, &group.ID, true, nil)
		reader := createJPEGMultipartReader(t, 300, 300, nil)
		resp1, err := server.UploadGroupLogo(ctx, genapi.UploadGroupLogoRequestObject{
			GroupId: group.ID,
			Body:    reader,
		})
		require.NoError(t, err)
		firstLogoURL := *resp1.(genapi.UploadGroupLogo200JSONResponse).LogoUrl

		// second logo
		mockAuth.ExpectCheckPermission(admin.ID, rbac.ManageGroupUsers, &group.ID, true, nil)
		reader2 := createJPEGMultipartReader(t, 300, 300, nil)
		resp2, err := server.UploadGroupLogo(ctx, genapi.UploadGroupLogoRequestObject{
			GroupId: group.ID,
			Body:    reader2,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadGroupLogo200JSONResponse{}, resp2)

		// different S3 key
		secondLogoURL := *resp2.(genapi.UploadGroupLogo200JSONResponse).LogoUrl
		assert.NotEqual(t, firstLogoURL, secondLogoURL)
	})
}
