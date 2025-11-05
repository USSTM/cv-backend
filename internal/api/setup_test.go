package api

import (
	"os"
	"testing"

	"github.com/USSTM/cv-backend/internal/testutil"
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
