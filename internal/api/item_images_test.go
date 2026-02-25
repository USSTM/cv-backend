package api

import (
	"bytes"
	"context"
	"image"
	"image/color"
	imgdraw "image/draw"
	"image/jpeg"
	"mime/multipart"
	"testing"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// creates an in-memory (multipart) reader with a JPEG image, expected as part of http upload request
func createJPEGMultipartReader(t *testing.T, w, h int, extraFields map[string]string) *multipart.Reader {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	imgdraw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 255, A: 255}}, image.Point{}, imgdraw.Src)
	var imgBuf bytes.Buffer
	require.NoError(t, jpeg.Encode(&imgBuf, img, nil))

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	fw, err := mw.CreateFormFile("image", "test.jpg")
	require.NoError(t, err)
	_, err = fw.Write(imgBuf.Bytes())
	require.NoError(t, err)

	for k, v := range extraFields {
		require.NoError(t, mw.WriteField(k, v))
	}
	require.NoError(t, mw.Close())

	return multipart.NewReader(&body, mw.Boundary())
}

func TestUploadItemImage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("success (non-primary image)", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("upload@item.ca").AsGlobalAdmin().Create()
		item := testDB.NewItem(t).WithName("Camera").WithType("medium").WithStock(5).Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		reader := createJPEGMultipartReader(t, 200, 150, map[string]string{
			"display_order": "1",
			"is_primary":    "false",
		})

		resp, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: item.ID,
			Body:   reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadItemImage201JSONResponse{}, resp)

		imgResp := resp.(genapi.UploadItemImage201JSONResponse)
		assert.Equal(t, item.ID, imgResp.ItemId)
		assert.NotEqual(t, uuid.Nil, imgResp.Id)
		assert.NotEmpty(t, imgResp.Url)
		assert.NotEmpty(t, imgResp.ThumbnailUrl)
		assert.False(t, imgResp.IsPrimary)
		assert.Equal(t, 1, imgResp.DisplayOrder)
	})

	t.Run("requires manage_items permission", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		member := testDB.NewUser(t).WithEmail("member@item.ca").AsMember().Create()
		item := testDB.NewItem(t).WithName("Camera2").WithType("medium").WithStock(5).Create()

		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageItems, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		reader := createJPEGMultipartReader(t, 200, 150, nil)
		resp, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: item.ID,
			Body:   reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadItemImage403JSONResponse{}, resp)
	})

	t.Run("item not found returns 404", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("notfound@item.ca").AsGlobalAdmin().Create()
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		reader := createJPEGMultipartReader(t, 200, 150, nil)
		resp, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: uuid.New(),
			Body:   reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadItemImage404JSONResponse{}, resp)
	})

	t.Run("succeeds with is_primary=true and square image", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("primary@item.ca").AsGlobalAdmin().Create()
		item := testDB.NewItem(t).WithName("Camera3").WithType("medium").WithStock(5).Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		reader := createJPEGMultipartReader(t, 300, 300, map[string]string{
			"is_primary":    "true",
			"display_order": "0",
		})

		resp, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: item.ID,
			Body:   reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadItemImage201JSONResponse{}, resp)

		imgResp := resp.(genapi.UploadItemImage201JSONResponse)
		assert.True(t, imgResp.IsPrimary)
	})

	t.Run("fails (400) with negative display_order", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("negorder@item.ca").AsGlobalAdmin().Create()
		item := testDB.NewItem(t).WithName("Camera5").WithType("medium").WithStock(5).Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		reader := createJPEGMultipartReader(t, 200, 150, map[string]string{
			"display_order": "-1",
		})
		resp, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: item.ID,
			Body:   reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadItemImage400JSONResponse{}, resp)
	})

	t.Run("fails (400) with display_order exceeding int32 max", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("maxorder@item.ca").AsGlobalAdmin().Create()
		item := testDB.NewItem(t).WithName("Camera6").WithType("medium").WithStock(5).Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		reader := createJPEGMultipartReader(t, 200, 150, map[string]string{
			"display_order": "2147483648", // math.MaxInt32 + 1
		})
		resp, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: item.ID,
			Body:   reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadItemImage400JSONResponse{}, resp)
	})

	t.Run("fails (400) with is_primary=true and non-square image", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("nonsquare@item.ca").AsGlobalAdmin().Create()
		item := testDB.NewItem(t).WithName("Camera4").WithType("medium").WithStock(5).Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		reader := createJPEGMultipartReader(t, 200, 150, map[string]string{
			"is_primary": "true",
		})

		resp, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: item.ID,
			Body:   reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadItemImage400JSONResponse{}, resp)
	})
}

func TestListItemImages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("returns all images for item", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("list@itemimg.ca").AsGlobalAdmin().Create()
		item := testDB.NewItem(t).WithName("ListCam").WithType("medium").WithStock(5).Create()

		// two images as sample
		for i := range 2 {
			mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
			ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())
			reader := createJPEGMultipartReader(t, 200, 150, map[string]string{"display_order": string(rune('0' + i))})
			_, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
				ItemId: item.ID,
				Body:   reader,
			})
			require.NoError(t, err)
		}

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ViewItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		resp, err := server.ListItemImages(ctx, genapi.ListItemImagesRequestObject{ItemId: item.ID})
		require.NoError(t, err)
		require.IsType(t, genapi.ListItemImages200JSONResponse{}, resp)

		// expect two images listed in item
		listResp := resp.(genapi.ListItemImages200JSONResponse)
		assert.Len(t, listResp, 2)
	})

	t.Run("requires view_items permission", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		member := testDB.NewUser(t).WithEmail("list-deny@item.ca").AsMember().Create()
		mockAuth.ExpectCheckPermission(member.ID, rbac.ViewItems, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		resp, err := server.ListItemImages(ctx, genapi.ListItemImagesRequestObject{ItemId: uuid.New()})
		require.NoError(t, err)
		require.IsType(t, genapi.ListItemImages403JSONResponse{}, resp)
	})

	t.Run("item not found returns 404", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("list-notfound@item.ca").AsGlobalAdmin().Create()
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ViewItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		resp, err := server.ListItemImages(ctx, genapi.ListItemImagesRequestObject{ItemId: uuid.New()})
		require.NoError(t, err)
		require.IsType(t, genapi.ListItemImages404JSONResponse{}, resp)
	})
}

func TestDeleteItemImage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("success with image deleted from S3 and DB", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("delete@itemimg.ca").AsGlobalAdmin().Create()
		item := testDB.NewItem(t).WithName("DelCam").WithType("medium").WithStock(5).Create()

		// Upload first
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())
		reader := createJPEGMultipartReader(t, 200, 150, nil)
		uploadResp, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: item.ID,
			Body:   reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadItemImage201JSONResponse{}, uploadResp)
		imageID := uploadResp.(genapi.UploadItemImage201JSONResponse).Id

		// Now delete
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		deleteResp, err := server.DeleteItemImage(ctx, genapi.DeleteItemImageRequestObject{
			ItemId:  item.ID,
			ImageId: imageID,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.DeleteItemImage204Response{}, deleteResp)

		// Verify image gone from DB
		_, err = testDB.Queries().GetItemImageByID(ctx, imageID)
		assert.Error(t, err)
	})

	t.Run("requires manage_items permission", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		member := testDB.NewUser(t).WithEmail("del-deny@item.ca").AsMember().Create()
		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageItems, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		resp, err := server.DeleteItemImage(ctx, genapi.DeleteItemImageRequestObject{
			ItemId:  uuid.New(),
			ImageId: uuid.New(),
		})
		require.NoError(t, err)
		require.IsType(t, genapi.DeleteItemImage403JSONResponse{}, resp)
	})

	t.Run("cannot delete image belonging to a different item", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("crossitem@itemimg.ca").AsGlobalAdmin().Create()
		itemA := testDB.NewItem(t).WithName("CrossItemA").WithType("medium").WithStock(5).Create()
		itemB := testDB.NewItem(t).WithName("CrossItemB").WithType("medium").WithStock(5).Create()

		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())
		reader := createJPEGMultipartReader(t, 200, 150, nil)
		uploadResp, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: itemA.ID,
			Body:   reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadItemImage201JSONResponse{}, uploadResp)
		imageID := uploadResp.(genapi.UploadItemImage201JSONResponse).Id

		// Attempt to delete item A image via item B endpoint
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		deleteResp, err := server.DeleteItemImage(ctx, genapi.DeleteItemImageRequestObject{
			ItemId:  itemB.ID,
			ImageId: imageID,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.DeleteItemImage404JSONResponse{}, deleteResp)
	})
}

func TestSetItemPrimaryImage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("success and sets primary, clears others", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("setprimary@item.ca").AsGlobalAdmin().Create()
		item := testDB.NewItem(t).WithName("PrimCam").WithType("medium").WithStock(5).Create()
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		// upload first square image
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		reader1 := createJPEGMultipartReader(t, 300, 300, nil)
		uploadResp1, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: item.ID,
			Body:   reader1,
		})
		require.NoError(t, err)
		imageID1 := uploadResp1.(genapi.UploadItemImage201JSONResponse).Id

		// upload second square image
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		reader2 := createJPEGMultipartReader(t, 300, 300, nil)
		uploadResp2, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: item.ID,
			Body:   reader2,
		})
		require.NoError(t, err)
		imageID2 := uploadResp2.(genapi.UploadItemImage201JSONResponse).Id

		// set first image as primary
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		resp1, err := server.SetItemPrimaryImage(ctx, genapi.SetItemPrimaryImageRequestObject{
			ItemId:  item.ID,
			ImageId: imageID1,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.SetItemPrimaryImage200JSONResponse{}, resp1)
		assert.True(t, resp1.(genapi.SetItemPrimaryImage200JSONResponse).IsPrimary)

		// set second image as primary
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		resp2, err := server.SetItemPrimaryImage(ctx, genapi.SetItemPrimaryImageRequestObject{
			ItemId:  item.ID,
			ImageId: imageID2,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.SetItemPrimaryImage200JSONResponse{}, resp2)
		assert.True(t, resp2.(genapi.SetItemPrimaryImage200JSONResponse).IsPrimary)

		// verify first image is no longer primary
		img1, err := testDB.Queries().GetItemImageByID(ctx, imageID1)
		require.NoError(t, err)
		assert.False(t, img1.IsPrimary, "first image should no longer be primary")
	})

	t.Run("rejects non-square image", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		adminUser := testDB.NewUser(t).WithEmail("nonsquareprimary@item.ca").AsGlobalAdmin().Create()
		item := testDB.NewItem(t).WithName("NsPrimCam").WithType("medium").WithStock(5).Create()
		ctx := testutil.ContextWithUser(context.Background(), adminUser, testDB.Queries())

		// non-square image
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		reader := createJPEGMultipartReader(t, 200, 150, nil)
		uploadResp, err := server.UploadItemImage(ctx, genapi.UploadItemImageRequestObject{
			ItemId: item.ID,
			Body:   reader,
		})
		require.NoError(t, err)
		imageID := uploadResp.(genapi.UploadItemImage201JSONResponse).Id

		// as primary should fail (non-square)
		mockAuth.ExpectCheckPermission(adminUser.ID, rbac.ManageItems, nil, true, nil)
		resp, err := server.SetItemPrimaryImage(ctx, genapi.SetItemPrimaryImageRequestObject{
			ItemId:  item.ID,
			ImageId: imageID,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.SetItemPrimaryImage400JSONResponse{}, resp)
	})

	t.Run("requires manage_items permission", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		member := testDB.NewUser(t).WithEmail("primary-deny@item.ca").AsMember().Create()
		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageItems, nil, false, nil)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		resp, err := server.SetItemPrimaryImage(ctx, genapi.SetItemPrimaryImageRequestObject{
			ItemId:  uuid.New(),
			ImageId: uuid.New(),
		})
		require.NoError(t, err)
		require.IsType(t, genapi.SetItemPrimaryImage403JSONResponse{}, resp)
	})
}
