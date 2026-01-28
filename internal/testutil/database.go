package testutil

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDatabase wraps a real PostgreSQL database for testing
type TestDatabase struct {
	*database.Database
	container testcontainers.Container
	pool      *pgxpool.Pool
	queries   *db.Queries
}

// NewTestDatabase creates a new test database using testcontainers
func NewTestDatabase(t *testing.T) *TestDatabase {
	ctx := context.Background()

	// Disable Ryuk (cleanup container) via environment variable
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	// Create PostgreSQL container with reuse enabled
	postgresContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithReuseByName("cv-backend-test-db"), // Enable container reuse across tests
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(30*time.Second),
				wait.ForListeningPort("5432/tcp").
					WithStartupTimeout(30*time.Second),
			),
		),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	// Create database connection - testcontainers wait strategy ensures it's ready
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err, "Failed to create connection pool")

	require.NoError(t, pool.Ping(ctx), "Failed to ping database")

	dbWrapper := &database.Database{}
	queries := db.New(pool)

	testDB := &TestDatabase{
		Database:  dbWrapper,
		container: postgresContainer,
		pool:      pool,
		queries:   queries,
	}

	return testDB
}

func (tdb *TestDatabase) Queries() *db.Queries {
	return tdb.queries
}

func (tdb *TestDatabase) Pool() *pgxpool.Pool {
	return tdb.pool
}

func (tdb *TestDatabase) RunMigrations(t *testing.T) {
	// Convert pgxpool connection to database/sql for goose
	sqlDB := stdlib.OpenDBFromPool(tdb.pool)
	defer sqlDB.Close()

	// Set the SQL dialect for goose
	goose.SetDialect("postgres")

	// Run migrations from db/migrations directory
	// relative from project root
	err := goose.Up(sqlDB, "../../db/migrations")
	require.NoError(t, err, "Failed to run goose migrations")
}

// Cleanup closes the database connection and terminates the container
func (tdb *TestDatabase) Cleanup() {
	ctx := context.Background()
	tdb.pool.Close()
	if err := tdb.container.Terminate(ctx); err != nil {
		// Log but don't fail tests on cleanup errors
	}
}

// CleanupDatabase truncates test data tables while preserving seed data
func (tdb *TestDatabase) CleanupDatabase(t *testing.T) {
	ctx := context.Background()

	// Only truncate tables with test data
	// Seed data tables (roles, permissions, role_permissions, time_slots) are preserved
	// Order matters: truncate child tables before parent tables to avoid FK violations
	tables := []string{
		"item_takings",      // references users, items
		"cart_items",        // references users, items, groups
		"booking",           // references users, items, user_availability
		"borrowings",        // references users, items, requests
		"requests",          // references users, items
		"user_availability", // references users, time_slots
		"user_roles",        // references users, roles, groups
		"signup_codes",      // references groups
		"items",             // no FK dependencies
		"users",             // no FK dependencies
		"groups",            // no FK dependencies
	}

	for _, table := range tables {
		_, err := tdb.pool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE")
		// Ignore errors if table doesn't exist yet (first test before migrations)
		if err != nil && !strings.Contains(err.Error(), "does not exist") {
			t.Logf("Failed to truncate table %s: %v", table, err)
		}
	}
}
