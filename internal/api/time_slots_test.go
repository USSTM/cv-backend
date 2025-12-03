package api

import (
	"context"
	"testing"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_ListTimeSlots(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	testDB := getSharedTestDatabase(t)
	mockJWT := testutil.NewMockJWTService(t)
	mockAuth := testutil.NewMockAuthenticator(t)
	server := NewServer(testDB, mockJWT, mockAuth)

	t.Run("successful list time slots", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("list@timeslot.ca").
			AsMember().
			Create()

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.ListTimeSlots(ctx, api.ListTimeSlotsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.ListTimeSlots200JSONResponse{}, response)

		slots := response.(api.ListTimeSlots200JSONResponse)
		assert.NotNil(t, slots)
		// 24 hours * 4 slots per hour
		assert.Len(t, slots, 96, "Expected 96 time slots")

		// first slot
		assert.Equal(t, "00:00:00", slots[0].StartTime)
		assert.Equal(t, "00:15:00", slots[0].EndTime)

		// last slot
		assert.Equal(t, "23:45:00", slots[95].StartTime)
		assert.Equal(t, "23:59:59", slots[95].EndTime)
	})

	t.Run("all roles can view time slots", func(t *testing.T) {
		roles := []struct {
			name    string
			builder func(*testutil.TestDatabase, *testing.T) *testutil.UserBuilder
		}{
			{"member", func(db *testutil.TestDatabase, t *testing.T) *testutil.UserBuilder {
				return db.NewUser(t).WithEmail("member@timeslot.ca").AsMember()
			}},
			{"group_admin", func(db *testutil.TestDatabase, t *testing.T) *testutil.UserBuilder {
				group := db.NewGroup(t).WithName("Time Slot Test Group").Create()
				return db.NewUser(t).WithEmail("groupadmin@timeslot.ca").AsGroupAdminOf(group)
			}},
			{"approver", func(db *testutil.TestDatabase, t *testing.T) *testutil.UserBuilder {
				return db.NewUser(t).WithEmail("approver@timeslot.ca").AsApprover()
			}},
			{"global_admin", func(db *testutil.TestDatabase, t *testing.T) *testutil.UserBuilder {
				return db.NewUser(t).WithEmail("admin@timeslot.ca").AsGlobalAdmin()
			}},
		}

		for _, role := range roles {
			t.Run(role.name, func(t *testing.T) {
				user := role.builder(testDB, t).Create()
				ctx := testutil.ContextWithUser(context.Background(), user, testDB.Queries())

				response, err := server.ListTimeSlots(ctx, api.ListTimeSlotsRequestObject{})

				require.NoError(t, err)
				require.IsType(t, api.ListTimeSlots200JSONResponse{}, response)

				slots := response.(api.ListTimeSlots200JSONResponse)
				assert.Len(t, slots, 96)
			})
		}
	})

	t.Run("unauthorized - not logged in", func(t *testing.T) {
		ctx := context.Background()

		response, err := server.ListTimeSlots(ctx, api.ListTimeSlotsRequestObject{})

		require.NoError(t, err)
		require.IsType(t, api.ListTimeSlots401JSONResponse{}, response)

		resp := response.(api.ListTimeSlots401JSONResponse)
		assert.Equal(t, int32(401), resp.Code)
		assert.Equal(t, "Unauthorized", resp.Message)
	})

	t.Run("time slot format validation", func(t *testing.T) {
		testUser := testDB.NewUser(t).
			WithEmail("format@timeslot.ca").
			AsMember().
			Create()

		ctx := testutil.ContextWithUser(context.Background(), testUser, testDB.Queries())

		response, err := server.ListTimeSlots(ctx, api.ListTimeSlotsRequestObject{})

		require.NoError(t, err)
		slots := response.(api.ListTimeSlots200JSONResponse)

		// Verify all times are in HH:MM:SS format
		for i, slot := range slots {
			assert.Regexp(t, `^\d{2}:\d{2}:\d{2}$`, slot.StartTime,
				"Slot %d start_time should be in HH:MM:SS format", i)
			assert.Regexp(t, `^\d{2}:\d{2}:\d{2}$`, slot.EndTime,
				"Slot %d end_time should be in HH:MM:SS format", i)
			assert.NotEmpty(t, slot.Id, "Slot %d should have an ID", i)
		}
	})
}
