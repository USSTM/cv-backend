package testutil

import (
	"context"
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

	// Create PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
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

	// Create database wrapper using your existing constructor
	dbWrapper := &database.Database{}
	// We'll need to access the pool directly for some operations
	// Since your Database struct has unexported fields, we'll work with queries directly
	queries := db.New(pool)

	testDB := &TestDatabase{
		Database:  dbWrapper,
		container: postgresContainer,
		pool:      pool,
		queries:   queries,
	}

	// Set up cleanup
	t.Cleanup(func() {
		pool.Close()
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	})

	return testDB
}

// Queries returns the SQLC queries interface
func (tdb *TestDatabase) Queries() *db.Queries {
	return tdb.queries
}

// RunMigrations runs your goose migrations
func (tdb *TestDatabase) RunMigrations(t *testing.T) {
	// Convert pgxpool connection to database/sql for goose
	sqlDB := stdlib.OpenDBFromPool(tdb.pool)
	defer sqlDB.Close()

	// Set the SQL dialect for goose
	goose.SetDialect("postgres")

	// Run migrations from your db/migrations directory
	// Need to use relative path from the project root
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

// CleanupDatabase truncates all tables for test isolation
func (tdb *TestDatabase) CleanupDatabase(t *testing.T) {
	ctx := context.Background()

	// Truncate in reverse dependency order based on your schema
	tables := []string{
		"booking",
		"user_availability",
		"time_slots",
		"borrowings",
		"requests",
		"cart",
		"items",
		"signup_codes",
		"user_roles",
		"role_permissions",
		"permissions",
		"roles",
		"users",
		"groups",
	}

	for _, table := range tables {
		_, err := tdb.pool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE")
		if err != nil {
			t.Logf("Failed to truncate table %s: %v", table, err)
		}
	}
}
