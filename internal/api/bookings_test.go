package api

import (
	"context"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/internal/rbac"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_GetBookingByID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, testQueue, mockJWT, mockAuth)

	t.Run("successful retrieval as booking owner", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@booking.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@booking.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		require.NotEmpty(t, timeSlots)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour) // 9 AM
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// User retrieves their own booking
		// Handler checks view_all_data permission even for owners
		mockAuth.ExpectCheckPermission(user.ID, rbac.ViewAllData, nil, false, nil)

		response, err := server.GetBookingByID(ctx, api.GetBookingByIDRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetBookingByID200JSONResponse{}, response)

		resp := response.(api.GetBookingByID200JSONResponse)
		assert.Equal(t, bookingID, resp.Id)
		assert.Equal(t, user.ID, resp.RequesterId)
		assert.Equal(t, item.ID, resp.ItemId)
		assert.Equal(t, "Main Office", resp.PickUpLocation)
		assert.Equal(t, api.RequestStatus("pending_confirmation"), resp.Status)
		assert.NotNil(t, resp.RequesterEmail)
		assert.Equal(t, user.Email, *resp.RequesterEmail)
	})

	t.Run("admin can view any booking", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@booking.test").AsMember().Create()
		admin := testDB.NewUser(t).WithEmail("admin@booking.test").AsGlobalAdmin().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@booking.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking for a different user
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user.ID, // Different user's booking
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// Admin retrieves another user's booking
		mockAuth.ExpectCheckPermission(admin.ID, rbac.ViewAllData, nil, true, nil)

		response, err := server.GetBookingByID(ctx, api.GetBookingByIDRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetBookingByID200JSONResponse{}, response)

		resp := response.(api.GetBookingByID200JSONResponse)
		assert.Equal(t, bookingID, resp.Id)
		assert.Equal(t, user.ID, resp.RequesterId)
	})

	t.Run("user cannot view another user's booking", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user1 := testDB.NewUser(t).WithEmail("user1@booking.test").AsMember().Create()
		user2 := testDB.NewUser(t).WithEmail("user2@booking.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@booking.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user2, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking for user1
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user1.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// user2 tries to view user1's booking
		mockAuth.ExpectCheckPermission(user2.ID, rbac.ViewAllData, nil, false, nil)

		response, err := server.GetBookingByID(ctx, api.GetBookingByIDRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.GetBookingByID403JSONResponse{}, response)
	})

	t.Run("booking not found", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		user := testDB.NewUser(t).WithEmail("user@booking.test").AsMember().Create()
		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		response, err := server.GetBookingByID(ctx, api.GetBookingByIDRequestObject{
			BookingId: uuid.New(),
		})

		require.NoError(t, err)
		require.IsType(t, api.GetBookingByID404JSONResponse{}, response)
	})

	t.Run("unauthorized", func(t *testing.T) {
		ctx := context.Background()

		response, err := server.GetBookingByID(ctx, api.GetBookingByIDRequestObject{
			BookingId: uuid.New(),
		})

		require.NoError(t, err)
		require.IsType(t, api.GetBookingByID401JSONResponse{}, response)
	})
}

func TestServer_GetMyBookings(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, testQueue, mockJWT, mockAuth)

	t.Run("successful retrieval of user's own bookings", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@bookings.test").AsMember().Create()
		otherUser := testDB.NewUser(t).WithEmail("other@bookings.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@bookings.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		require.NotEmpty(t, timeSlots)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create 2 bookings for user
		booking1ID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             booking1ID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		booking2ID := uuid.New()
		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             booking2ID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate.Add(24 * time.Hour), Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate.Add(24 * time.Hour), Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusConfirmed,
		})
		require.NoError(t, err)

		// Create 1 booking for other user
		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             uuid.New(),
			RequesterID:    &otherUser.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// Get user's bookings
		response, err := server.GetMyBookings(ctx, api.GetMyBookingsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetMyBookings200JSONResponse{}, response)

		resp := response.(api.GetMyBookings200JSONResponse)
		assert.Len(t, resp, 2) // Returned only user's bookings

		// Verify IDs
		bookingIDs := []uuid.UUID{resp[0].Id, resp[1].Id}
		assert.Contains(t, bookingIDs, booking1ID)
		assert.Contains(t, bookingIDs, booking2ID)
	})

	t.Run("filter by status", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		user := testDB.NewUser(t).WithEmail("user@bookings.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@bookings.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		// Create confirmed booking
		confirmedID := uuid.New()
		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             confirmedID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusConfirmed,
		})
		require.NoError(t, err)

		// Create pending booking
		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             uuid.New(),
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate.Add(24 * time.Hour), Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate.Add(24 * time.Hour), Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// Filter by confirmed status
		confirmedStatus := api.RequestStatus("confirmed")
		response, err := server.GetMyBookings(ctx, api.GetMyBookingsRequestObject{
			Params: api.GetMyBookingsParams{
				Status: &confirmedStatus,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.GetMyBookings200JSONResponse{}, response)

		resp := response.(api.GetMyBookings200JSONResponse)
		assert.Len(t, resp, 1)
		assert.Equal(t, confirmedID, resp[0].Id)
		assert.Equal(t, api.RequestStatus("confirmed"), resp[0].Status)
	})

	t.Run("empty results when user has no bookings", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		user := testDB.NewUser(t).WithEmail("user@bookings.test").AsMember().Create()
		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		response, err := server.GetMyBookings(ctx, api.GetMyBookingsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetMyBookings200JSONResponse{}, response)

		resp := response.(api.GetMyBookings200JSONResponse)
		assert.Len(t, resp, 0)
	})

	t.Run("unauthorized", func(t *testing.T) {
		ctx := context.Background()

		response, err := server.GetMyBookings(ctx, api.GetMyBookingsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.GetMyBookings401JSONResponse{}, response)
	})
}

func TestServer_ListPendingConfirmation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, testQueue, mockJWT, mockAuth)

	t.Run("approver can view pending confirmations", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		approver := testDB.NewUser(t).WithEmail("approver@test.com").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()
		user := testDB.NewUser(t).WithEmail("user@test.com").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()

		ctx := testutil.ContextWithUser(context.Background(), approver, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		require.NotEmpty(t, timeSlots)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create pending confirmation booking
		pendingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             pendingID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// Create confirmed booking (should not appear)
		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             uuid.New(),
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate.Add(24 * time.Hour), Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate.Add(24 * time.Hour), Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusConfirmed,
		})
		require.NoError(t, err)

		// Approver views pending confirmations
		mockAuth.ExpectCheckPermission(approver.ID, rbac.ManageAllBookings, nil, true, nil)

		response, err := server.ListPendingConfirmation(ctx, api.ListPendingConfirmationRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.ListPendingConfirmation200JSONResponse{}, response)

		resp := response.(api.ListPendingConfirmation200JSONResponse)
		assert.Len(t, resp, 1)
		assert.Equal(t, pendingID, resp[0].Id)
		assert.Equal(t, api.RequestStatus("pending_confirmation"), resp[0].Status)
	})

	t.Run("user with manage_group_bookings can view pending confirmations", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Use global admin
		manager := testDB.NewUser(t).WithEmail("manager@test.com").AsGlobalAdmin().Create()
		user := testDB.NewUser(t).WithEmail("user@test.com").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@test.com").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), manager, testDB.Queries())

		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		pendingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             pendingID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// User with manage_group_bookings permission can view (must provide group_id)
		mockAuth.ExpectCheckPermission(manager.ID, rbac.ManageAllBookings, nil, false, nil)
		mockAuth.ExpectCheckPermission(manager.ID, rbac.ManageGroupBookings, &group.ID, true, nil)

		response, err := server.ListPendingConfirmation(ctx, api.ListPendingConfirmationRequestObject{
			Params: api.ListPendingConfirmationParams{
				GroupId: &group.ID,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ListPendingConfirmation200JSONResponse{}, response)

		resp := response.(api.ListPendingConfirmation200JSONResponse)
		assert.Len(t, resp, 1)
		assert.Equal(t, pendingID, resp[0].Id)
	})

	t.Run("member cannot view pending confirmations", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		member := testDB.NewUser(t).WithEmail("member@test.com").AsMember().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()
		ctx := testutil.ContextWithUser(context.Background(), member, testDB.Queries())

		// Member lacks permissions (even with group_id)
		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageAllBookings, nil, false, nil)
		mockAuth.ExpectCheckPermission(member.ID, rbac.ManageGroupBookings, &group.ID, false, nil)

		response, err := server.ListPendingConfirmation(ctx, api.ListPendingConfirmationRequestObject{
			Params: api.ListPendingConfirmationParams{
				GroupId: &group.ID,
			},
		})

		require.NoError(t, err)
		require.IsType(t, api.ListPendingConfirmation403JSONResponse{}, response)
	})

	t.Run("empty results when no pending confirmations", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		approver := testDB.NewUser(t).WithEmail("approver@test.com").AsApprover().Create()
		ctx := testutil.ContextWithUser(context.Background(), approver, testDB.Queries())

		mockAuth.ExpectCheckPermission(approver.ID, rbac.ManageAllBookings, nil, true, nil)

		response, err := server.ListPendingConfirmation(ctx, api.ListPendingConfirmationRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.ListPendingConfirmation200JSONResponse{}, response)

		resp := response.(api.ListPendingConfirmation200JSONResponse)
		assert.Len(t, resp, 0)
	})

	t.Run("unauthorized", func(t *testing.T) {
		ctx := context.Background()

		response, err := server.ListPendingConfirmation(ctx, api.ListPendingConfirmationRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.ListPendingConfirmation401JSONResponse{}, response)
	})
}

func TestServer_ListBookings(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, testQueue, mockJWT, mockAuth)

	t.Run("admin can view all bookings", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		admin := testDB.NewUser(t).WithEmail("admin@test.com").AsGlobalAdmin().Create()
		user1 := testDB.NewUser(t).WithEmail("user1@test.com").AsMember().Create()
		user2 := testDB.NewUser(t).WithEmail("user2@test.com").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@test.com").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		// Create booking for user1
		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             uuid.New(),
			RequesterID:    &user1.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// Create booking for user2
		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             uuid.New(),
			RequesterID:    &user2.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate.Add(24 * time.Hour), Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate.Add(24 * time.Hour), Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusConfirmed,
		})
		require.NoError(t, err)

		// Admin views all bookings
		mockAuth.ExpectCheckPermission(admin.ID, rbac.ViewAllData, nil, true, nil)

		response, err := server.ListBookings(ctx, api.ListBookingsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.ListBookings200JSONResponse{}, response)

		resp := response.(api.ListBookings200JSONResponse)
		assert.Len(t, resp, 2) // Both bookings visible
	})

	t.Run("regular user only sees own bookings", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		user := testDB.NewUser(t).WithEmail("user@test.com").AsMember().Create()
		otherUser := testDB.NewUser(t).WithEmail("other@test.com").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@test.com").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		// Create booking for user
		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             uuid.New(),
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// Create booking for other user
		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             uuid.New(),
			RequesterID:    &otherUser.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate.Add(24 * time.Hour), Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate.Add(24 * time.Hour), Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusConfirmed,
		})
		require.NoError(t, err)

		// User only sees their own booking
		mockAuth.ExpectCheckPermission(user.ID, rbac.ViewAllData, nil, false, nil)

		response, err := server.ListBookings(ctx, api.ListBookingsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.ListBookings200JSONResponse{}, response)

		resp := response.(api.ListBookings200JSONResponse)
		assert.Len(t, resp, 1) // Only user's booking visible
		assert.Equal(t, user.ID, resp[0].RequesterId)
	})
}

func TestServer_ConfirmBooking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, testQueue, mockJWT, mockAuth)

	t.Run("success - requester confirms booking within 48h before pickup", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@confirm.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@confirm.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		require.NotEmpty(t, timeSlots)
		timeSlotID := timeSlots[0].ID

		// Create availability (7 days in future)
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking (pending_confirmation, created just now)
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour) // 9 AM on future date
		returnDate := pickupDate.Add(24 * time.Hour)

		booking, err := testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// User confirms booking
		response, err := server.ConfirmBooking(ctx, api.ConfirmBookingRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.ConfirmBooking200JSONResponse{}, response)

		resp := response.(api.ConfirmBooking200JSONResponse)
		assert.Equal(t, bookingID, resp.Id)
		assert.Equal(t, api.RequestStatus("confirmed"), resp.Status)
		assert.NotNil(t, resp.ConfirmedAt)
		assert.NotNil(t, resp.ConfirmedBy)
		assert.Equal(t, user.ID, *resp.ConfirmedBy)

		// Verify database state
		updatedBooking, err := testDB.Queries().GetBookingByID(ctx, bookingID)
		require.NoError(t, err)
		assert.Equal(t, db.RequestStatusConfirmed, updatedBooking.Status)
		assert.True(t, updatedBooking.ConfirmedAt.Valid)
		assert.NotNil(t, updatedBooking.ConfirmedBy)
		assert.Equal(t, user.ID, *updatedBooking.ConfirmedBy)

		// Verify original booking created_at is recent
		timeSinceCreation := time.Since(booking.CreatedAt.Time)
		assert.Less(t, timeSinceCreation, 1*time.Minute, "Booking should be recently created")
	})

	t.Run("unauthorized - no authentication", func(t *testing.T) {
		ctx := context.Background()

		response, err := server.ConfirmBooking(ctx, api.ConfirmBookingRequestObject{
			BookingId: uuid.New(),
		})

		require.NoError(t, err)
		require.IsType(t, api.ConfirmBooking401JSONResponse{}, response)

		resp := response.(api.ConfirmBooking401JSONResponse)
		assert.Equal(t, "AUTHENTICATION_REQUIRED", string(resp.Error.Code))
		assert.Equal(t, "Authentication required", resp.Error.Message)
	})

	t.Run("not found - invalid booking ID", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		user := testDB.NewUser(t).WithEmail("user@confirm.test").AsMember().Create()
		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		response, err := server.ConfirmBooking(ctx, api.ConfirmBookingRequestObject{
			BookingId: uuid.New(),
		})

		require.NoError(t, err)
		require.IsType(t, api.ConfirmBooking404JSONResponse{}, response)

		resp := response.(api.ConfirmBooking404JSONResponse)
		assert.Equal(t, "RESOURCE_NOT_FOUND", string(resp.Error.Code))
		assert.Equal(t, "Booking not found", resp.Error.Message)
	})

	t.Run("forbidden - different user tries to confirm", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user1 := testDB.NewUser(t).WithEmail("user1@confirm.test").AsMember().Create()
		user2 := testDB.NewUser(t).WithEmail("user2@confirm.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@confirm.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user2, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking for user1
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user1.ID, // Belongs to user1
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// user2 tries to confirm user1's booking
		response, err := server.ConfirmBooking(ctx, api.ConfirmBookingRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.ConfirmBooking403JSONResponse{}, response)

		resp := response.(api.ConfirmBooking403JSONResponse)
		assert.Equal(t, "PERMISSION_DENIED", string(resp.Error.Code))
		assert.Equal(t, "Only the requester can confirm this booking", resp.Error.Message)
	})

	t.Run("bad request - wrong status (already confirmed)", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@confirm.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@confirm.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking that's already confirmed
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusConfirmed, // Already confirmed
		})
		require.NoError(t, err)

		// User tries to confirm already-confirmed booking
		response, err := server.ConfirmBooking(ctx, api.ConfirmBookingRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.ConfirmBooking400JSONResponse{}, response)

		resp := response.(api.ConfirmBooking400JSONResponse)
		assert.Equal(t, "VALIDATION_ERROR", string(resp.Error.Code))
		assert.Equal(t, "Booking is not in pending_confirmation status", resp.Error.Message)
	})

	t.Run("bad request - 48h window expired", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@confirm.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@confirm.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability (far in future so pickup date isn't an issue)
		futureDate := time.Now().AddDate(0, 0, 14) // 14 days in future
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking with created_at = 49 hours ago
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)
		createdAt := time.Now().Add(-49 * time.Hour) // 49 hours ago

		// SQL insert to set created_at manually
		_, err = testDB.Pool().Exec(ctx, `
			INSERT INTO booking (id, requester_id, manager_id, item_id, group_id, availability_id, pick_up_date, pick_up_location, return_date, return_location, status, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, bookingID, user.ID, approver.ID, item.ID, group.ID, availability.ID, pickupDate, "Main Office", returnDate, "Main Office", db.RequestStatusPendingConfirmation, createdAt)
		require.NoError(t, err)

		// User tries to confirm after 48h window
		response, err := server.ConfirmBooking(ctx, api.ConfirmBookingRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.ConfirmBooking400JSONResponse{}, response)

		resp := response.(api.ConfirmBooking400JSONResponse)
		assert.Equal(t, "VALIDATION_ERROR", string(resp.Error.Code))
		assert.Equal(t, "Confirmation window expired (must confirm within 48 hours)", resp.Error.Message)
	})

	t.Run("bad request - after pickup date passed", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@confirm.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@confirm.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability in the past
		pastDate := time.Now().AddDate(0, 0, -7) // 7 days ago
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: pastDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking with pickup date in the past
		bookingID := uuid.New()
		pickupDate := pastDate.Add(9 * time.Hour) // 9 AM on past date
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// User tries to confirm after pickup date
		response, err := server.ConfirmBooking(ctx, api.ConfirmBookingRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.ConfirmBooking400JSONResponse{}, response)

		resp := response.(api.ConfirmBooking400JSONResponse)
		assert.Equal(t, "VALIDATION_ERROR", string(resp.Error.Code))
		assert.Equal(t, "Cannot confirm booking after pickup date has passed", resp.Error.Message)
	})

	t.Run("idempotency - already confirmed", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@confirm.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@confirm.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// First confirmation succeed
		response1, err := server.ConfirmBooking(ctx, api.ConfirmBookingRequestObject{
			BookingId: bookingID,
		})
		require.NoError(t, err)
		require.IsType(t, api.ConfirmBooking200JSONResponse{}, response1)

		// Second confirmation fail with status
		response2, err := server.ConfirmBooking(ctx, api.ConfirmBookingRequestObject{
			BookingId: bookingID,
		})
		require.NoError(t, err)
		require.IsType(t, api.ConfirmBooking400JSONResponse{}, response2)

		resp := response2.(api.ConfirmBooking400JSONResponse)
		assert.Equal(t, "VALIDATION_ERROR", string(resp.Error.Code))
		assert.Equal(t, "Booking is not in pending_confirmation status", resp.Error.Message)
	})
}

func TestServer_CancelBooking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, testQueue, mockJWT, mockAuth)

	t.Run("success - requester cancels before pickup", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@cancel.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@cancel.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		require.NotEmpty(t, timeSlots)
		timeSlotID := timeSlots[0].ID

		// Create availability (future date)
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// User cancels booking before pickup
		mockAuth.ExpectCheckPermission(user.ID, rbac.ManageAllBookings, nil, false, nil)

		response, err := server.CancelBooking(ctx, api.CancelBookingRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.CancelBooking200JSONResponse{}, response)

		resp := response.(api.CancelBooking200JSONResponse)
		assert.Equal(t, bookingID, resp.Id)
		assert.Equal(t, api.RequestStatus("cancelled"), resp.Status)

		// Verify database state
		updatedBooking, err := testDB.Queries().GetBookingByID(ctx, bookingID)
		require.NoError(t, err)
		assert.Equal(t, db.RequestStatusCancelled, updatedBooking.Status)
	})

	t.Run("success - manager cancels before pickup", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@cancel.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		manager := testDB.NewUser(t).WithEmail("manager@cancel.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), manager, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &manager.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking for user
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user.ID,
			ManagerID:      &manager.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// Manager cancels user's booking
		mockAuth.ExpectCheckPermission(manager.ID, rbac.ManageAllBookings, nil, true, nil)

		response, err := server.CancelBooking(ctx, api.CancelBookingRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.CancelBooking200JSONResponse{}, response)

		resp := response.(api.CancelBooking200JSONResponse)
		assert.Equal(t, bookingID, resp.Id)
		assert.Equal(t, api.RequestStatus("cancelled"), resp.Status)
	})

	t.Run("success - manager cancels after pickup (admin override)", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@cancel.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		admin := testDB.NewUser(t).WithEmail("admin@cancel.test").AsGlobalAdmin().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), admin, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability in the past
		pastDate := time.Now().AddDate(0, 0, -7) // 7 days ago
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &admin.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: pastDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking with pickup date in the past
		bookingID := uuid.New()
		pickupDate := pastDate.Add(9 * time.Hour) // Past date
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user.ID,
			ManagerID:      &admin.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusConfirmed,
		})
		require.NoError(t, err)

		// Admin cancels booking after pickup date (admin override)
		mockAuth.ExpectCheckPermission(admin.ID, rbac.ManageAllBookings, nil, true, nil)

		response, err := server.CancelBooking(ctx, api.CancelBookingRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.CancelBooking200JSONResponse{}, response)

		resp := response.(api.CancelBooking200JSONResponse)
		assert.Equal(t, bookingID, resp.Id)
		assert.Equal(t, api.RequestStatus("cancelled"), resp.Status)
	})

	t.Run("forbidden - requester cancels after pickup", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@cancel.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@cancel.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability in the past
		pastDate := time.Now().AddDate(0, 0, -7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: pastDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking with pickup date in the past
		bookingID := uuid.New()
		pickupDate := pastDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusConfirmed,
		})
		require.NoError(t, err)

		// User tries to cancel after pickup date
		mockAuth.ExpectCheckPermission(user.ID, rbac.ManageAllBookings, nil, false, nil)

		response, err := server.CancelBooking(ctx, api.CancelBookingRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.CancelBooking403JSONResponse{}, response)

		resp := response.(api.CancelBooking403JSONResponse)
		assert.Equal(t, "PERMISSION_DENIED", string(resp.Error.Code))
		assert.Equal(t, "Insufficient permissions to cancel this booking", resp.Error.Message)
	})

	t.Run("forbidden - different user without permissions", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user1 := testDB.NewUser(t).WithEmail("user1@cancel.test").AsMember().Create()
		user2 := testDB.NewUser(t).WithEmail("user2@cancel.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@cancel.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user2, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking for user1
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user1.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// user2 tries to cancel user1's booking
		mockAuth.ExpectCheckPermission(user2.ID, rbac.ManageAllBookings, nil, false, nil)

		response, err := server.CancelBooking(ctx, api.CancelBookingRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.CancelBooking403JSONResponse{}, response)

		resp := response.(api.CancelBooking403JSONResponse)
		assert.Equal(t, "PERMISSION_DENIED", string(resp.Error.Code))
		assert.Equal(t, "Insufficient permissions to cancel this booking", resp.Error.Message)
	})

	t.Run("not found - invalid booking ID", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		user := testDB.NewUser(t).WithEmail("user@cancel.test").AsMember().Create()
		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		response, err := server.CancelBooking(ctx, api.CancelBookingRequestObject{
			BookingId: uuid.New(),
		})

		require.NoError(t, err)
		require.IsType(t, api.CancelBooking404JSONResponse{}, response)

		resp := response.(api.CancelBooking404JSONResponse)
		assert.Equal(t, "RESOURCE_NOT_FOUND", string(resp.Error.Code))
		assert.Equal(t, "Booking not found", resp.Error.Message)
	})

	t.Run("unauthorized - no authentication", func(t *testing.T) {
		ctx := context.Background()

		response, err := server.CancelBooking(ctx, api.CancelBookingRequestObject{
			BookingId: uuid.New(),
		})

		require.NoError(t, err)
		require.IsType(t, api.CancelBooking401JSONResponse{}, response)

		resp := response.(api.CancelBooking401JSONResponse)
		assert.Equal(t, "AUTHENTICATION_REQUIRED", string(resp.Error.Code))
		assert.Equal(t, "Authentication required", resp.Error.Message)
	})

	t.Run("cancel already cancelled booking (idempotent)", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user := testDB.NewUser(t).WithEmail("user@cancel.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@cancel.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// Cancel once
		mockAuth.ExpectCheckPermission(user.ID, rbac.ManageAllBookings, nil, false, nil)
		response1, err := server.CancelBooking(ctx, api.CancelBookingRequestObject{
			BookingId: bookingID,
		})
		require.NoError(t, err)
		require.IsType(t, api.CancelBooking200JSONResponse{}, response1)

		// Cancel again
		mockAuth.ExpectCheckPermission(user.ID, rbac.ManageAllBookings, nil, false, nil)
		response2, err := server.CancelBooking(ctx, api.CancelBookingRequestObject{
			BookingId: bookingID,
		})
		require.NoError(t, err)
		require.IsType(t, api.CancelBooking200JSONResponse{}, response2)

		resp := response2.(api.CancelBooking200JSONResponse)
		assert.Equal(t, api.RequestStatus("cancelled"), resp.Status)
	})

	t.Run("user without manage_all_bookings permission", func(t *testing.T) {
		testDB.CleanupDatabase(t)

		// Create test data
		user1 := testDB.NewUser(t).WithEmail("user1@cancel.test").AsMember().Create()
		user2 := testDB.NewUser(t).WithEmail("user2@cancel.test").AsMember().Create()
		item := testDB.NewItem(t).WithName("Laptop").WithType("high").WithStock(5).Create()
		approver := testDB.NewUser(t).WithEmail("approver@cancel.test").AsApprover().Create()
		group := testDB.NewGroup(t).WithName("Test Group").Create()

		ctx := testutil.ContextWithUser(context.Background(), user2, testDB.Queries())

		// Get a time slot
		timeSlots, _ := testDB.Queries().ListTimeSlots(ctx)
		timeSlotID := timeSlots[0].ID

		// Create availability
		futureDate := time.Now().AddDate(0, 0, 7)
		availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &approver.ID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: futureDate, Valid: true},
		})
		require.NoError(t, err)

		// Create booking for user1
		bookingID := uuid.New()
		pickupDate := futureDate.Add(9 * time.Hour)
		returnDate := pickupDate.Add(24 * time.Hour)

		_, err = testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
			ID:             bookingID,
			RequesterID:    &user1.ID,
			ManagerID:      &approver.ID,
			ItemID:         &item.ID,
			GroupID:        &group.ID,
			AvailabilityID: &availability.ID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: "Main Office",
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: "Main Office",
			Status:         db.RequestStatusPendingConfirmation,
		})
		require.NoError(t, err)

		// user2 (not requester, no permissions) tries to cancel
		mockAuth.ExpectCheckPermission(user2.ID, rbac.ManageAllBookings, nil, false, nil)

		response, err := server.CancelBooking(ctx, api.CancelBookingRequestObject{
			BookingId: bookingID,
		})

		require.NoError(t, err)
		require.IsType(t, api.CancelBooking403JSONResponse{}, response)

		resp := response.(api.CancelBooking403JSONResponse)
		assert.Equal(t, "PERMISSION_DENIED", string(resp.Error.Code))
	})
}
