package api

import (
	"bytes"
	"context"
	"mime/multipart"
	"testing"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/image"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func multipartReaderWithImage(t *testing.T, name string, data []byte, fields map[string]string) *multipart.Reader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", name)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	require.NoError(t, writer.Close())
	return multipart.NewReader(&body, writer.Boundary())
}

func TestUploadPreCheckoutConditionImage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("member can upload and use returned key as beforeConditionUrl", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)
		member := testDB.NewUser(t).WithEmail("pre-checkout-upload@borrowimg.ca").AsMember().Create()
		item := testDB.NewItem(t).WithName("Pre-checkout camera").WithType("medium").WithStock(1).Create()
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())
		mockAuth.ExpectCheckPermission(member.ID, rbac.ViewItems, nil, true, nil)

		resp, err := server.UploadPreCheckoutConditionImage(ctx, genapi.UploadPreCheckoutConditionImageRequestObject{
			Body: createJPEGMultipartReader(t, 200, 150, map[string]string{"item_id": item.ID.String()}),
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadPreCheckoutConditionImage201JSONResponse{}, resp)

		body := resp.(genapi.UploadPreCheckoutConditionImage201JSONResponse)
		assert.Equal(t, item.ID, body.ItemId)
		assert.NotEmpty(t, body.BeforeConditionUrl)
		assert.NotEmpty(t, body.PreviewUrl)
		assert.NotEqual(t, body.BeforeConditionUrl, body.PreviewUrl)
		assert.Regexp(t, "^pre-checkout-condition-images/"+member.ID.String()+"/"+item.ID.String()+"/.+\\.jpg$", body.BeforeConditionUrl)
		object, err := sharedLocalStack.GetObject(ctx, body.BeforeConditionUrl)
		require.NoError(t, err)
		require.NoError(t, object.Close())
		assert.NotEqual(t, body.BeforeConditionUrl, server.conditionImageURL(ctx, body.BeforeConditionUrl))
	})

	t.Run("rejects a non-image upload", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)
		member := testDB.NewUser(t).WithEmail("pre-checkout-invalid@borrowimg.ca").AsMember().Create()
		item := testDB.NewItem(t).WithName("Invalid upload camera").WithType("medium").WithStock(1).Create()
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())
		mockAuth.ExpectCheckPermission(member.ID, rbac.ViewItems, nil, true, nil)

		resp, err := server.UploadPreCheckoutConditionImage(ctx, genapi.UploadPreCheckoutConditionImageRequestObject{
			Body: multipartReaderWithImage(t, "notes.txt", []byte("not an image"), map[string]string{"item_id": item.ID.String()}),
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadPreCheckoutConditionImage400JSONResponse{}, resp)
	})

	t.Run("rejects an image larger than the shared size limit", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)
		member := testDB.NewUser(t).WithEmail("pre-checkout-large@borrowimg.ca").AsMember().Create()
		item := testDB.NewItem(t).WithName("Large upload camera").WithType("medium").WithStock(1).Create()
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())
		mockAuth.ExpectCheckPermission(member.ID, rbac.ViewItems, nil, true, nil)

		resp, err := server.UploadPreCheckoutConditionImage(ctx, genapi.UploadPreCheckoutConditionImageRequestObject{
			Body: multipartReaderWithImage(t, "large.jpg", make([]byte, image.MaxFileSize+1), map[string]string{"item_id": item.ID.String()}),
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadPreCheckoutConditionImage400JSONResponse{}, resp)
	})

	t.Run("requires a valid item_id", func(t *testing.T) {
		server, testDB, _ := newTestServer(t)
		member := testDB.NewUser(t).WithEmail("pre-checkout-missing-item@borrowimg.ca").AsMember().Create()
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		resp, err := server.UploadPreCheckoutConditionImage(ctx, genapi.UploadPreCheckoutConditionImageRequestObject{
			Body: createJPEGMultipartReader(t, 200, 150, map[string]string{"item_id": uuid.NewString()}),
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadPreCheckoutConditionImage404JSONResponse{}, resp)
	})
}
