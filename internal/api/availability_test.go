package api

import (
	"context"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toOpenAPIDate(t time.Time) openapi_types.Date {
	return openapi_types.Date{Time: t}
}

func TestServer_CreateAvailability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, mockJWT, mockAuth)

	t.Run("successful create availability as approver", func(t *testing.T) {
		testDB.CleanupDatabase(t)
		approver := testDB.NewUser(t).WithEmail("approver@avail.test").AsApprover().Create()
		ctx := testutil.ContextWithUser(context.Background(), approver, testDB.Queries())

		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		require.NotEmpty(t, timeSlots)
		timeSlotID := timeSlots[0].ID

		futureDate := toOpenAPIDate(time.Now().AddDate(0, 0, 7))

		mockAuth.ExpectCheckPermission(approver.ID, "manage_time_slots", nil, true, nil)

		response, err := server.CreateAvailability(ctx, api.CreateAvailabilityRequestObject{
			Body: &api.CreateAvailabilityRequest{
				TimeSlotId: timeSlotID,
				Date:       futureDate,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CreateAvailability201JSONResponse{}, response)

		resp := response.(api.CreateAvailability201JSONResponse)
		assert.Equal(t, approver.ID, resp.UserId)
		assert.Equal(t, timeSlotID, resp.TimeSlotId)
		assert.Equal(t, futureDate.Time.Format("2006-01-02"), resp.Date.Time.Format("2006-01-02"))
		assert.NotEmpty(t, resp.StartTime)
		assert.NotEmpty(t, resp.EndTime)
	})

	t.Run("member cannot create availability", func(t *testing.T) {
		testDB.CleanupDatabase(t)
		member := testDB.NewUser(t).WithEmail("member@avail.test").AsMember().Create()
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID
		futureDate := toOpenAPIDate(time.Now().AddDate(0, 0, 7))

		mockAuth.ExpectCheckPermission(member.ID, "manage_time_slots", nil, false, nil)

		response, err := server.CreateAvailability(ctx, api.CreateAvailabilityRequestObject{
			Body: &api.CreateAvailabilityRequest{
				TimeSlotId: timeSlotID,
				Date:       futureDate,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CreateAvailability403JSONResponse{}, response)
	})

	t.Run("date in the past", func(t *testing.T) {
		testDB.CleanupDatabase(t)
		approver := testDB.NewUser(t).WithEmail("approver2@avail.test").AsApprover().Create()
		ctx := testutil.ContextWithUser(context.Background(), approver, testDB.Queries())

		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID
		pastDate := toOpenAPIDate(time.Now().AddDate(0, 0, -7))

		mockAuth.ExpectCheckPermission(approver.ID, "manage_time_slots", nil, true, nil)

		response, err := server.CreateAvailability(ctx, api.CreateAvailabilityRequestObject{
			Body: &api.CreateAvailabilityRequest{
				TimeSlotId: timeSlotID,
				Date:       pastDate,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CreateAvailability400JSONResponse{}, response)

		resp := response.(api.CreateAvailability400JSONResponse)
		assert.Contains(t, resp.Message, "must be in the future")
	})

	t.Run("duplicate availability", func(t *testing.T) {
		testDB.CleanupDatabase(t)
		approver := testDB.NewUser(t).WithEmail("approver3@avail.test").AsApprover().Create()
		ctx := testutil.ContextWithUser(context.Background(), approver, testDB.Queries())

		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID
		futureDate := toOpenAPIDate(time.Now().AddDate(0, 0, 7))

		parsedDate := futureDate.Time
		_, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: parsedDate, Valid: true},
		})
		require.NoError(t, err)

		// duplicate
		mockAuth.ExpectCheckPermission(approver.ID, "manage_time_slots", nil, true, nil)

		response, err := server.CreateAvailability(ctx, api.CreateAvailabilityRequestObject{
			Body: &api.CreateAvailabilityRequest{
				TimeSlotId: timeSlotID,
				Date:       futureDate,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CreateAvailability409JSONResponse{}, response)

		resp := response.(api.CreateAvailability409JSONResponse)
		assert.Contains(t, resp.Message, "already have availability")
	})

	t.Run("unauthorized", func(t *testing.T) {
		ctx := context.Background()

		response, err := server.CreateAvailability(ctx, api.CreateAvailabilityRequestObject{
			Body: &api.CreateAvailabilityRequest{
				TimeSlotId: uuid.New(),
				Date:       toOpenAPIDate(time.Now().AddDate(0, 0, 7)),
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.CreateAvailability401JSONResponse{}, response)
	})
}

func TestServer_ListAvailability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testDB.CleanupDatabase(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, mockJWT, mockAuth)

	approver1 := testDB.NewUser(t).WithEmail("approver1@list.test").AsApprover().Create()
	approver2 := testDB.NewUser(t).WithEmail("approver2@list.test").AsApprover().Create()
	member := testDB.NewUser(t).WithEmail("member@list.test").AsMember().Create()

	ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())
	timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)

	date1 := time.Now().AddDate(0, 0, 7)
	date2 := time.Now().AddDate(0, 0, 14)

	// approver1 on date1
	testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
		ID:         uuid.New(),
		UserID:     &approver1.ID,
		TimeSlotID: &timeSlots[0].ID,
		Date:       pgtype.Date{Time: date1, Valid: true},
	})

	// approver2 on date2
	testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
		ID:         uuid.New(),
		UserID:     &approver2.ID,
		TimeSlotID: &timeSlots[1].ID,
		Date:       pgtype.Date{Time: date2, Valid: true},
	})

	t.Run("list all availability", func(t *testing.T) {
		response, err := server.ListAvailability(ctx, api.ListAvailabilityRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.ListAvailability200JSONResponse{}, response)

		resp := response.(api.ListAvailability200JSONResponse)
		assert.Len(t, resp, 2)
	})

	t.Run("filter by date", func(t *testing.T) {
		dateFilter := toOpenAPIDate(date1)
		response, err := server.ListAvailability(ctx, api.ListAvailabilityRequestObject{
			Params: api.ListAvailabilityParams{
				Date: &dateFilter,
			},
		})

		require.NoError(t, err)
		resp := response.(api.ListAvailability200JSONResponse)
		assert.Len(t, resp, 1)
		assert.Equal(t, dateFilter.Time.Format("2006-01-02"), resp[0].Date.Time.Format("2006-01-02"))
	})

	t.Run("filter by user_id", func(t *testing.T) {
		response, err := server.ListAvailability(ctx, api.ListAvailabilityRequestObject{
			Params: api.ListAvailabilityParams{
				UserId: &approver1.ID,
			},
		})

		require.NoError(t, err)
		resp := response.(api.ListAvailability200JSONResponse)
		assert.Len(t, resp, 1)
		assert.Equal(t, approver1.ID, resp[0].UserId)
	})

	t.Run("fail - unauthorized", func(t *testing.T) {
		ctx := context.Background()
		response, err := server.ListAvailability(ctx, api.ListAvailabilityRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.ListAvailability401JSONResponse{}, response)
	})
}

func TestServer_GetAvailabilityByDate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testDB.CleanupDatabase(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, mockJWT, mockAuth)

	approver := testDB.NewUser(t).WithEmail("approver@date.test").AsApprover().Create()
	member := testDB.NewUser(t).WithEmail("member@date.test").AsMember().Create()
	ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

	timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
	targetDate := time.Now().AddDate(0, 0, 7)

	testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
		ID:         uuid.New(),
		UserID:     &approver.ID,
		TimeSlotID: &timeSlots[0].ID,
		Date:       pgtype.Date{Time: targetDate, Valid: true},
	})

	t.Run("get availability by date", func(t *testing.T) {
		dateStr := toOpenAPIDate(targetDate)
		response, err := server.GetAvailabilityByDate(ctx, api.GetAvailabilityByDateRequestObject{
			Date: dateStr,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetAvailabilityByDate200JSONResponse{}, response)

		resp := response.(api.GetAvailabilityByDate200JSONResponse)
		assert.Len(t, resp, 1)
		assert.Equal(t, dateStr.Time.Format("2006-01-02"), resp[0].Date.Time.Format("2006-01-02"))
		assert.Equal(t, approver.ID, resp[0].UserId)
	})

	t.Run("date with no availability empty", func(t *testing.T) {
		emptyDate := toOpenAPIDate(time.Now().AddDate(0, 0, 30))
		response, err := server.GetAvailabilityByDate(ctx, api.GetAvailabilityByDateRequestObject{
			Date: emptyDate,
		})

		require.NoError(t, err)
		resp := response.(api.GetAvailabilityByDate200JSONResponse)
		assert.Empty(t, resp)
	})

}

func TestServer_GetAvailabilityByID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testDB.CleanupDatabase(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, mockJWT, mockAuth)

	approver := testDB.NewUser(t).WithEmail("approver@id.test").AsApprover().Create()
	member := testDB.NewUser(t).WithEmail("member@id.test").AsMember().Create()
	ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

	timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
	targetDate := time.Now().AddDate(0, 0, 7)

	availability, _ := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
		ID:         uuid.New(),
		UserID:     &approver.ID,
		TimeSlotID: &timeSlots[0].ID,
		Date:       pgtype.Date{Time: targetDate, Valid: true},
	})

	t.Run("get availability by ID", func(t *testing.T) {
		response, err := server.GetAvailabilityByID(ctx, api.GetAvailabilityByIDRequestObject{
			Id: availability.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetAvailabilityByID200JSONResponse{}, response)

		resp := response.(api.GetAvailabilityByID200JSONResponse)
		assert.Equal(t, availability.ID, resp.Id)
		assert.Equal(t, approver.ID, resp.UserId)
	})

	t.Run("availability not found", func(t *testing.T) {
		response, err := server.GetAvailabilityByID(ctx, api.GetAvailabilityByIDRequestObject{
			Id: uuid.New(),
		})

		require.NoError(t, err)
		require.IsType(t, api.GetAvailabilityByID404JSONResponse{}, response)
	})
}

func TestServer_GetUserAvailability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testDB.CleanupDatabase(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, mockJWT, mockAuth)

	approver := testDB.NewUser(t).WithEmail("approver@user.test").AsApprover().Create()

	ctx := testutil.ContextWithUser(context.Background(), approver, testDB.Queries())
	timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)

	date1 := time.Now().AddDate(0, 0, 7)
	date2 := time.Now().AddDate(0, 0, 14)

	testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
		ID:         uuid.New(),
		UserID:     &approver.ID,
		TimeSlotID: &timeSlots[0].ID,
		Date:       pgtype.Date{Time: date1, Valid: true},
	})

	testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
		ID:         uuid.New(),
		UserID:     &approver.ID,
		TimeSlotID: &timeSlots[1].ID,
		Date:       pgtype.Date{Time: date2, Valid: true},
	})

	t.Run("user views own availability", func(t *testing.T) {
		response, err := server.GetUserAvailability(ctx, api.GetUserAvailabilityRequestObject{
			UserId: approver.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserAvailability200JSONResponse{}, response)

		resp := response.(api.GetUserAvailability200JSONResponse)
		assert.Len(t, resp, 2)
	})

	t.Run("any user can view other availability", func(t *testing.T) {
		otherUser := testDB.NewUser(t).WithEmail("other@user.test").AsMember().Create()
		otherCtx := testutil.ContextWithUser(context.Background(), otherUser, testDB.Queries())

		response, err := server.GetUserAvailability(otherCtx, api.GetUserAvailabilityRequestObject{
			UserId: approver.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetUserAvailability200JSONResponse{}, response)

		resp := response.(api.GetUserAvailability200JSONResponse)
		assert.Len(t, resp, 2)
	})

	t.Run("filter by date range", func(t *testing.T) {
		fromDate := toOpenAPIDate(date1.AddDate(0, 0, -1))
		toDateParam := toOpenAPIDate(date1.AddDate(0, 0, 1))

		response, err := server.GetUserAvailability(ctx, api.GetUserAvailabilityRequestObject{
			UserId: approver.ID,
			Params: api.GetUserAvailabilityParams{
				FromDate: &fromDate,
				ToDate:   &toDateParam,
			},
		})

		require.NoError(t, err)
		resp := response.(api.GetUserAvailability200JSONResponse)
		assert.Len(t, resp, 1)
	})
}

func TestServer_DeleteAvailability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testDB.CleanupDatabase(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, mockJWT, mockAuth)

	approver := testDB.NewUser(t).WithEmail("approver@delete.test").AsApprover().Create()
	member := testDB.NewUser(t).WithEmail("member@delete.test").AsMember().Create()

	ctx := testutil.ContextWithUser(context.Background(), approver, testDB.Queries())
	timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
	targetDate := time.Now().AddDate(0, 0, 7)

	t.Run("delete availability", func(t *testing.T) {
		availability, _ := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlots[0].ID,
			Date:       pgtype.Date{Time: targetDate, Valid: true},
		})

		mockAuth.ExpectCheckPermission(approver.ID, "manage_time_slots", nil, true, nil)

		response, err := server.DeleteAvailability(ctx, api.DeleteAvailabilityRequestObject{
			Id: availability.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.DeleteAvailability204Response{}, response)

		// Verify deleted
		_, err = testDB.Queries().GetAvailabilityByID(ctx, availability.ID)
		assert.Error(t, err)
	})

	t.Run("member cannot delete availability", func(t *testing.T) {
		availability, _ := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlots[1].ID,
			Date:       pgtype.Date{Time: targetDate, Valid: true},
		})

		memberCtx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())
		mockAuth.ExpectCheckPermission(member.ID, "manage_time_slots", nil, false, nil)

		response, err := server.DeleteAvailability(memberCtx, api.DeleteAvailabilityRequestObject{
			Id: availability.ID,
		})

		require.NoError(t, err)
		require.IsType(t, api.DeleteAvailability403JSONResponse{}, response)
	})
}
