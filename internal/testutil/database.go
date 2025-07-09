package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"
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
	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
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

// SeedTestData seeds the database with basic test data
func (tdb *TestDatabase) SeedTestData(t *testing.T) {
	ctx := context.Background()

	// Create basic roles
	require.NoError(t, tdb.queries.CreateRole(ctx, db.CreateRoleParams{
		Name:        "student",
		Description: pgtype.Text{String: "Student role", Valid: true},
	}))

	require.NoError(t, tdb.queries.CreateRole(ctx, db.CreateRoleParams{
		Name:        "admin",
		Description: pgtype.Text{String: "Administrator role", Valid: true},
	}))

	// Create basic permissions
	require.NoError(t, tdb.queries.CreatePermission(ctx, db.CreatePermissionParams{
		Name:        "view_own_data",
		Description: pgtype.Text{String: "Can view own data", Valid: true},
	}))

	require.NoError(t, tdb.queries.CreatePermission(ctx, db.CreatePermissionParams{
		Name:        "view_items",
		Description: pgtype.Text{String: "Can view items", Valid: true},
	}))

	// Link roles to permissions
	require.NoError(t, tdb.queries.CreateRolePermission(ctx, db.CreateRolePermissionParams{
		RoleName:       "student",
		PermissionName: "view_own_data",
	}))

	require.NoError(t, tdb.queries.CreateRolePermission(ctx, db.CreateRolePermissionParams{
		RoleName:       "student",
		PermissionName: "view_items",
	}))

	require.NoError(t, tdb.queries.CreateRolePermission(ctx, db.CreateRolePermissionParams{
		RoleName:       "admin",
		PermissionName: "view_own_data",
	}))

	require.NoError(t, tdb.queries.CreateRolePermission(ctx, db.CreateRolePermissionParams{
		RoleName:       "admin",
		PermissionName: "view_items",
	}))
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

// CreateTestUser creates a test user using your existing SQLC queries
func (tdb *TestDatabase) CreateTestUser(t *testing.T, user *TestUser) {
	ctx := context.Background()

	// Create user using your existing CreateUser query
	// Hash the password "password" using bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	require.NoError(t, err, "Failed to hash password")

	dbUser, err := tdb.Queries().CreateUser(ctx, db.CreateUserParams{
		Email:        user.Email,
		PasswordHash: string(hashedPassword),
	})
	require.NoError(t, err, "Failed to create test user")

	// Update the user ID to match what was created
	user.ID = dbUser.ID

	// Assign roles using your existing CreateUserRole query
	for _, roleName := range user.Roles {
		require.NoError(t, tdb.Queries().CreateUserRole(ctx, db.CreateUserRoleParams{
			UserID:   &dbUser.ID,
			RoleName: pgtype.Text{String: roleName, Valid: true},
			Scope:    "global",
			ScopeID:  nil,
		}))
	}
}

// CreateTestItem creates a test item using your existing SQLC queries
func (tdb *TestDatabase) CreateTestItem(t *testing.T, item TestItem) uuid.UUID {
	ctx := context.Background()

	dbItem, err := tdb.Queries().CreateItem(ctx, db.CreateItemParams{
		Name:        item.Name,
		Description: pgtype.Text{String: item.Description, Valid: item.Description != ""},
		Type:        db.ItemType(item.Type),
		Stock:       int32(item.Stock),
	})
	require.NoError(t, err, "Failed to create test item")

	return dbItem.ID
}

// TestItem represents a test item for database seeding
type TestItem struct {
	Name        string
	Description string
	Type        string // "low", "medium", "high" per your schema
	Stock       int
	URLs        []string
}

// NewTestItem creates a test item with default values
func NewTestItem() TestItem {
	return TestItem{
		Name:        "Test Item",
		Description: "Test item description",
		Type:        "low",
		Stock:       10,
		URLs:        []string{},
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
