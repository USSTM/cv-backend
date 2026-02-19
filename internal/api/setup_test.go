package api

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/require"
)

var (
	sharedTestDB     *testutil.TestDatabase
	sharedQueue      *testutil.TestQueue
	sharedLocalStack *testutil.TestLocalStack
)

// TestMain runs once before all tests
func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(0)
	}

	t := &testing.T{}

	sharedTestDB = testutil.NewTestDatabase(t)
	sharedTestDB.RunMigrations(t)
	sharedQueue = testutil.NewTestQueue(t)
	sharedLocalStack = testutil.NewTestLocalStack(t)

	code := m.Run()

	if sharedTestDB.Pool() != nil {
		sharedTestDB.Pool().Close()
	}
	sharedLocalStack.Close()
	sharedQueue.Close()

	os.Exit(code)
}

// getSharedTestDatabase returns the shared test database with clean tables
func getSharedTestDatabase(t *testing.T) *testutil.TestDatabase {
	// Truncate tables to give each test a fresh state
	sharedTestDB.CleanupDatabase(t)
	return sharedTestDB
}

// wrap around newAuthTestServer without breaking current tests
func newTestServer(t *testing.T) (*Server, *testutil.TestDatabase, *testutil.MockAuthenticator) {
	s, db, mock, _ := newAuthTestServer(t)
	return s, db, mock
}

// newAuthTestServer is like newTestServer but also returns the AuthService so auth
// handler tests can set up state without going through email.
func newAuthTestServer(t *testing.T) (*Server, *testutil.TestDatabase, *testutil.MockAuthenticator, *auth.AuthService) {
	testDB := getSharedTestDatabase(t)
	sharedQueue.Cleanup(t)

	jwtSvc, err := auth.NewJWTService([]byte("test-signing-key"), "test-issuer", 15*time.Minute)
	require.NoError(t, err)

	authSvc := auth.NewAuthService(sharedQueue.Redis, jwtSvc, testDB.Queries(), config.AuthConfig{
		OTPExpiry:      5 * time.Minute,
		OTPCooldown:    60 * time.Second,
		OTPMaxAttempts: 3,
		RefreshExpiry:  7 * 24 * time.Hour,
	})

	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, sharedQueue, authSvc, mockAuth, sharedLocalStack, sharedLocalStack)
	return server, testDB, mockAuth, authSvc
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
