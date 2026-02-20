package api

import (
	"context"
	"testing"
	"time"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// creates a borrowing record directly in the DB for test setup.
func createBorrowing(t *testing.T, testDB *testutil.TestDatabase, userID uuid.UUID) db.Borrowing {
	t.Helper()
	ctx := context.Background()

	group := testDB.NewGroup(t).WithName("BorrowGroup-" + uuid.New().String()[:8]).Create()
	item := testDB.NewItem(t).WithName("BorrowItem-" + uuid.New().String()[:8]).WithType("medium").WithStock(5).Create()

	borrowing, err := testDB.Queries().BorrowItem(ctx, db.BorrowItemParams{
		UserID:             &userID,
		GroupID:            &group.ID,
		ID:                 item.ID,
		Quantity:           1,
		DueDate:            pgtype.Timestamp{Time: time.Now().Add(7 * 24 * time.Hour), Valid: true},
		BeforeCondition:    "good",
		BeforeConditionUrl: "",
	})
	require.NoError(t, err)
	return borrowing
}

func TestUploadBorrowingImage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("success for 'own borrowing' with 'request_items' permission", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		member := testDB.NewUser(t).WithEmail("upload@borrowimg.ca").AsMember().Create()
		borrowing := createBorrowing(t, testDB, member.ID)

		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageAllBookings, nil, false, nil)
		mockAuth.ExpectCheckPermission(member.ID, rbac.RequestItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		reader := createJPEGMultipartReader(t, 200, 150, map[string]string{"image_type": "before"})
		resp, err := server.UploadBorrowingImage(ctx, genapi.UploadBorrowingImageRequestObject{
			BorrowingId: borrowing.ID,
			Body:        reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadBorrowingImage201JSONResponse{}, resp)

		imgResp := resp.(genapi.UploadBorrowingImage201JSONResponse)
		assert.Equal(t, borrowing.ID, imgResp.BorrowingId)
		assert.NotEmpty(t, imgResp.Url)
		assert.Equal(t, genapi.BorrowingImageImageTypeBefore, imgResp.ImageType)
	})

	t.Run("success for any borrowing with 'manage_all_bookings' permission", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		approver := testDB.NewUser(t).WithEmail("approver@borrowimg.ca").AsApprover().Create()
		otherUser := testDB.NewUser(t).WithEmail("other@borrowimg.ca").AsMember().Create()
		borrowing := createBorrowing(t, testDB, otherUser.ID)

		mockAuth.ExpectCheckPermission(approver.ID, rbac.ManageAllBookings, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), approver, testDB.Queries())

		reader := createJPEGMultipartReader(t, 200, 150, map[string]string{"image_type": "after"})
		resp, err := server.UploadBorrowingImage(ctx, genapi.UploadBorrowingImageRequestObject{
			BorrowingId: borrowing.ID,
			Body:        reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadBorrowingImage201JSONResponse{}, resp)
	})

	t.Run("denied for another user's borrowing with 'request_items' permission", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		member := testDB.NewUser(t).WithEmail("member@borrowimg.ca").AsMember().Create()
		otherUser := testDB.NewUser(t).WithEmail("owner@borrowimg.ca").AsMember().Create()
		borrowing := createBorrowing(t, testDB, otherUser.ID)

		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageAllBookings, nil, false, nil)
		mockAuth.ExpectCheckPermission(member.ID, rbac.RequestItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		reader := createJPEGMultipartReader(t, 200, 150, map[string]string{"image_type": "before"})
		resp, err := server.UploadBorrowingImage(ctx, genapi.UploadBorrowingImageRequestObject{
			BorrowingId: borrowing.ID,
			Body:        reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadBorrowingImage403JSONResponse{}, resp)
	})

	t.Run("missing image_type returns 400", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		member := testDB.NewUser(t).WithEmail("notype@borrowimg.ca").AsMember().Create()
		borrowing := createBorrowing(t, testDB, member.ID)

		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageAllBookings, nil, false, nil)
		mockAuth.ExpectCheckPermission(member.ID, rbac.RequestItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		reader := createJPEGMultipartReader(t, 200, 150, nil) // no image_type
		resp, err := server.UploadBorrowingImage(ctx, genapi.UploadBorrowingImageRequestObject{
			BorrowingId: borrowing.ID,
			Body:        reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadBorrowingImage400JSONResponse{}, resp)
	})

	t.Run("invalid image_type returns 400", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		member := testDB.NewUser(t).WithEmail("badtype@borrowimg.ca").AsMember().Create()
		borrowing := createBorrowing(t, testDB, member.ID)

		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageAllBookings, nil, false, nil)
		mockAuth.ExpectCheckPermission(member.ID, rbac.RequestItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		reader := createJPEGMultipartReader(t, 200, 150, map[string]string{"image_type": "invalid"})
		resp, err := server.UploadBorrowingImage(ctx, genapi.UploadBorrowingImageRequestObject{
			BorrowingId: borrowing.ID,
			Body:        reader,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.UploadBorrowingImage400JSONResponse{}, resp)
	})
}

func TestListBorrowingImages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("returns all images for borrowing", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		member := testDB.NewUser(t).WithEmail("list@borrowimg.ca").AsMember().Create()
		borrowing := createBorrowing(t, testDB, member.ID)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		// Upload two images
		for _, imgType := range []string{"before", "after"} {
			mockAuth.ExpectCheckPermission(member.ID, rbac.ManageAllBookings, nil, false, nil)
			mockAuth.ExpectCheckPermission(member.ID, rbac.RequestItems, nil, true, nil)
			reader := createJPEGMultipartReader(t, 200, 150, map[string]string{"image_type": imgType})
			_, err := server.UploadBorrowingImage(ctx, genapi.UploadBorrowingImageRequestObject{
				BorrowingId: borrowing.ID,
				Body:        reader,
			})
			require.NoError(t, err)
		}

		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageAllBookings, nil, false, nil)
		mockAuth.ExpectCheckPermission(member.ID, rbac.RequestItems, nil, true, nil)
		resp, err := server.ListBorrowingImages(ctx, genapi.ListBorrowingImagesRequestObject{
			BorrowingId: borrowing.ID,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.ListBorrowingImages200JSONResponse{}, resp)
		assert.Len(t, resp.(genapi.ListBorrowingImages200JSONResponse), 2)
	})
}

func TestDeleteBorrowingImage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("success removes S3 object and DB record", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		member := testDB.NewUser(t).WithEmail("delete@borrowimg.ca").AsMember().Create()
		borrowing := createBorrowing(t, testDB, member.ID)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageAllBookings, nil, false, nil)
		mockAuth.ExpectCheckPermission(member.ID, rbac.RequestItems, nil, true, nil)
		reader := createJPEGMultipartReader(t, 200, 150, map[string]string{"image_type": "before"})
		uploadResp, err := server.UploadBorrowingImage(ctx, genapi.UploadBorrowingImageRequestObject{
			BorrowingId: borrowing.ID,
			Body:        reader,
		})
		require.NoError(t, err)
		imageID := uploadResp.(genapi.UploadBorrowingImage201JSONResponse).Id

		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageAllBookings, nil, false, nil)
		mockAuth.ExpectCheckPermission(member.ID, rbac.RequestItems, nil, true, nil)
		deleteResp, err := server.DeleteBorrowingImage(ctx, genapi.DeleteBorrowingImageRequestObject{
			BorrowingId: borrowing.ID,
			ImageId:     imageID,
		})
		require.NoError(t, err)
		require.IsType(t, genapi.DeleteBorrowingImage204Response{}, deleteResp)

		_, err = testDB.Queries().GetBorrowingImageByID(ctx, imageID)
		assert.Error(t, err)
	})

	t.Run("denied for another user's borrowing", func(t *testing.T) {
		server, testDB, mockAuth := newTestServer(t)

		member := testDB.NewUser(t).WithEmail("deldenied@borrowimg.ca").AsMember().Create()
		otherUser := testDB.NewUser(t).WithEmail("delowner@borrowimg.ca").AsMember().Create()
		borrowing := createBorrowing(t, testDB, otherUser.ID)

		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageAllBookings, nil, false, nil)
		mockAuth.ExpectCheckPermission(member.ID, rbac.RequestItems, nil, true, nil)
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		resp, err := server.DeleteBorrowingImage(ctx, genapi.DeleteBorrowingImageRequestObject{
			BorrowingId: borrowing.ID,
			ImageId:     uuid.New(),
		})
		require.NoError(t, err)
		require.IsType(t, genapi.DeleteBorrowingImage403JSONResponse{}, resp)
	})
}
