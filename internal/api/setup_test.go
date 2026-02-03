package api

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/require"
)

var (
	sharedTestDB *testutil.TestDatabase
)

// TestMain runs once before all tests
func TestMain(m *testing.M) {
	// Create container and pool
	sharedTestDB = testutil.NewTestDatabase(&testing.T{})

	// Run migrations once
	sharedTestDB.RunMigrations(&testing.T{})

	// Run all tests
	code := m.Run()

	// Cleanup
	if sharedTestDB.Pool() != nil {
		sharedTestDB.Pool().Close()
	}

	os.Exit(code)
}

// getSharedTestDatabase returns the shared test database with clean tables
func getSharedTestDatabase(t *testing.T) *testutil.TestDatabase {
	// Truncate tables to give each test a fresh state
	sharedTestDB.CleanupDatabase(t)
	return sharedTestDB
}

// test server initializer
func newTestServer(t *testing.T) (*Server, *testutil.TestDatabase, *testutil.MockAuthenticator) {
	testDB := getSharedTestDatabase(t)
	testQueue := testutil.NewTestQueue(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	mockEmail := testutil.NewTestLocalStack(t)
	server := NewServer(testDB, testQueue, mockJWT, mockAuth, mockEmail)
	return server, testDB, mockAuth
}

func toOpenAPIDate(t time.Time) openapi_types.Date {
	return openapi_types.Date{Time: t}
}

// createTestAvailability creates an availability record for the given approver using
// the first seeded time slot, 7 days in the future.
func createTestAvailability(t *testing.T, testDB *testutil.TestDatabase, approverID uuid.UUID) db.UserAvailability {
	t.Helper()
	ctx := context.Background()
	timeSlots, err := testDB.Queries().ListTimeSlots(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, timeSlots)

	futureDate := time.Now().AddDate(0, 0, 7)
	availability, err := testDB.Queries().CreateAvailability(ctx, db.CreateAvailabilityParams{
		ID:         uuid.New(),
		UserID:     &approverID,
		TimeSlotID: &timeSlots[0].ID,
		Date:       pgtype.Date{Time: futureDate, Valid: true},
	})
	require.NoError(t, err)
	return availability
}

type testBooking struct {
	ID             uuid.UUID
	PickupDate     time.Time
	ReturnDate     time.Time
	AvailabilityID uuid.UUID
}

// createTestBooking inserts a booking row. pickupOffset lets caller stagger multiple
// bookings off the same base date (pass 0 for the default 9 AM pickup).
func createTestBooking(t *testing.T, testDB *testutil.TestDatabase,
	availabilityID, requesterID, approverID, itemID, groupID uuid.UUID,
	status db.RequestStatus, pickupOffset time.Duration) testBooking {

	t.Helper()
	ctx := context.Background()

	futureDate := time.Now().AddDate(0, 0, 7)
	pickupDate := futureDate.Add(9*time.Hour + pickupOffset)
	returnDate := pickupDate.Add(24 * time.Hour)
	bookingID := uuid.New()

	_, err := testDB.Queries().CreateBooking(ctx, db.CreateBookingParams{
		ID:             bookingID,
		RequesterID:    &requesterID,
		ManagerID:      &approverID,
		ItemID:         &itemID,
		GroupID:        &groupID,
		AvailabilityID: &availabilityID,
		PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
		PickUpLocation: "Main Office",
		ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
		ReturnLocation: "Main Office",
		Status:         status,
	})
	require.NoError(t, err)

	return testBooking{
		ID:             bookingID,
		PickupDate:     pickupDate,
		ReturnDate:     returnDate,
		AvailabilityID: availabilityID,
	}
}
