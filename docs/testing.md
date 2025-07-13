# Testing Guide

This guide covers testing patterns and utilities available in the project.

## Overview

The project uses a comprehensive testing approach with:
- **Integration tests** using real PostgreSQL databases via testcontainers
- **Unit tests** for business logic
- **Builder pattern** for test data creation
- **Mock services** for external dependencies
- **StrictServerInterface testing** for direct handler testing

## Test Database Setup

### Using TestDatabase

The project provides `testutil.TestDatabase` that spins up a real PostgreSQL container for each test:

```go
func TestMyFeature(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Create test database with container
    testDB := testutil.NewTestDatabase(t)
    testDB.RunMigrations(t) // Runs all migrations + seeds

    // Test database is automatically cleaned up when test ends
}
```

### Running Migrations

Migrations are automatically run with `testDB.RunMigrations(t)`, which:
1. Applies all SQL migrations from `db/migrations/`
2. Seeds roles and permissions from `20250626025312_seed_roles_permissions.sql`
3. Sets up the complete database schema

## Test Data Builders

### Creating Test Users

The builder pattern provides a fluent API for creating test users with proper role scoping:

```go
// Basic member user
testUser := testDB.NewUser(t).
    WithEmail("test@example.com").
    AsMember().
    Create()

// Global admin
adminUser := testDB.NewUser(t).
    WithEmail("admin@example.com").
    AsGlobalAdmin().
    Create()

// Approver with scheduling permissions
approverUser := testDB.NewUser(t).
    WithEmail("approver@example.com").
    AsApprover().
    Create()
```

### Creating Test Groups

Groups are required for group-scoped roles:

```go
// Create a test group
group := testDB.NewGroup(t).
    WithName("Engineering Group").
    WithDescription("Test engineering group").
    Create()

// Create group admin for this group
groupAdmin := testDB.NewUser(t).
    WithEmail("groupadmin@example.com").
    AsGroupAdminOf(group).
    Create()

// Create member of this group
groupMember := testDB.NewUser(t).
    WithEmail("member@example.com").
    AsMemberOf(group).
    Create()
```

### Creating Test Items

```go
// Basic item
itemID := testDB.NewItem(t).
    WithName("Test Item").
    WithDescription("A test item").
    WithType("low").
    WithStock(10).
    Create()

// High-priority item requiring approval
highPriorityID := testDB.NewItem(t).
    WithName("Expensive Equipment").
    WithType("high").
    WithStock(1).
    Create()
```

### Available User Roles

The system has four main roles based on the database migration:

#### Global Admin
- **Scope**: Global
- **Permissions**: All permissions
- **Usage**: `AsGlobalAdmin()`

```go
admin := testDB.NewUser(t).AsGlobalAdmin().Create()
```

#### Approver  
- **Scope**: Global
- **Permissions**: Approval and scheduling permissions
- **Usage**: `AsApprover()`

```go
approver := testDB.NewUser(t).AsApprover().Create()
```

#### Group Admin
- **Scope**: Group-specific
- **Permissions**: Group management permissions (no approval rights)
- **Usage**: `AsGroupAdminOf(group)`

```go
group := testDB.NewGroup(t).Create()
groupAdmin := testDB.NewUser(t).AsGroupAdminOf(group).Create()
```

#### Member
- **Scope**: Global or Group-specific
- **Permissions**: Basic permissions (view_items, manage_cart, request_items, view_own_data)
- **Usage**: `AsMember()` or `AsMemberOf(group)`

```go
// Global member
member := testDB.NewUser(t).AsMember().Create()

// Group member
group := testDB.NewGroup(t).Create()
groupMember := testDB.NewUser(t).AsMemberOf(group).Create()
```

#### Custom Roles
For testing edge cases:

```go
// Custom role with specific scope
user := testDB.NewUser(t).
    WithCustomRole("custom_role", "global", nil).
    Create()

// Group-scoped custom role
user := testDB.NewUser(t).
    WithCustomRole("custom_role", "group", &group.ID).
    Create()
```

## Testing Handlers

### Direct Handler Testing

With StrictServerInterface, you can test handlers directly without HTTP mocking:

```go
func TestCreateItem(t *testing.T) {
    // Setup
    testDB := testutil.NewTestDatabase(t)
    testDB.RunMigrations(t)
    
    mockJWT := testutil.NewMockJWTService(t)
    mockAuth := testutil.NewMockAuthenticator(t)
    server := NewServer(testDB, mockJWT, mockAuth)

    testUser := testDB.NewUser(t).AsGlobalAdmin().Create()

    t.Run("successful creation", func(t *testing.T) {
        // Setup permissions mock
        mockAuth.ExpectCheckPermission(testUser.ID, "manage_items", nil, true, nil)

        // Create authenticated context
        ctx := context.WithValue(context.Background(), auth.UserClaimsKey, &auth.AuthenticatedUser{
            ID:    testUser.ID,
            Email: testUser.Email,
        })

        // Create request object
        request := api.CreateItemRequestObject{
            Body: &api.CreateItemJSONRequestBody{
                Name:        "Test Item",
                Description: stringPtr("Test description"),
                Type:        api.ItemTypeLow,
                Stock:       10,
            },
        }

        // Call handler directly
        response, err := server.CreateItem(ctx, request)

        // Assert response
        require.NoError(t, err)
        require.IsType(t, api.CreateItem201JSONResponse{}, response)

        createResp := response.(api.CreateItem201JSONResponse)
        assert.Equal(t, "Test Item", *createResp.Name)
        assert.NotNil(t, createResp.Id)

        // Verify mocks
        mockAuth.AssertExpectations(t)
    })
}

// Helper function for string pointers
func stringPtr(s string) *string {
    return &s
}
```

### Testing Authentication and Authorization

```go
func TestAuthenticationRequired(t *testing.T) {
    testDB := testutil.NewTestDatabase(t)
    testDB.RunMigrations(t)
    
    server := NewServer(testDB, nil, nil)

    t.Run("unauthenticated request", func(t *testing.T) {
        // No user in context
        ctx := context.Background()
        
        response, err := server.GetItems(ctx, api.GetItemsRequestObject{})
        
        require.NoError(t, err)
        require.IsType(t, api.GetItems401JSONResponse{}, response)
        
        errorResp := response.(api.GetItems401JSONResponse)
        assert.Equal(t, int32(401), errorResp.Code)
    })
}

func TestInsufficientPermissions(t *testing.T) {
    testDB := testutil.NewTestDatabase(t)
    testDB.RunMigrations(t)
    
    mockAuth := testutil.NewMockAuthenticator(t)
    server := NewServer(testDB, nil, mockAuth)
    
    testUser := testDB.NewUser(t).AsMember().Create()

    t.Run("insufficient permissions", func(t *testing.T) {
        // Mock permission check to return false
        mockAuth.ExpectCheckPermission(testUser.ID, "manage_items", nil, false, nil)

        ctx := context.WithValue(context.Background(), auth.UserClaimsKey, &auth.AuthenticatedUser{
            ID:    testUser.ID,
            Email: testUser.Email,
        })

        response, err := server.CreateItem(ctx, api.CreateItemRequestObject{
            Body: &api.CreateItemJSONRequestBody{
                Name:  "Test",
                Type:  api.ItemTypeLow,
                Stock: 1,
            },
        })

        require.NoError(t, err)
        require.IsType(t, api.CreateItem403JSONResponse{}, response)
        
        mockAuth.AssertExpectations(t)
    })
}
```

## Mock Services

### JWT Service Mock

```go
func TestWithJWTMock(t *testing.T) {
    mockJWT := testutil.NewMockJWTService(t)
    
    userID := uuid.New()
    
    // Setup expectations
    mockJWT.ExpectGenerateToken(userID, "test-token", nil)
    
    // Your test code here...
    
    // Verify expectations were met
    mockJWT.AssertExpectations(t)
}
```

### Authenticator Mock

```go
func TestWithAuthMock(t *testing.T) {
    mockAuth := testutil.NewMockAuthenticator(t)
    
    userID := uuid.New()
    
    // Setup permission expectations
    mockAuth.ExpectCheckPermission(userID, "view_items", nil, true, nil)
    mockAuth.ExpectCheckPermission(userID, "manage_items", nil, false, nil)
    
    // Your test code here...
    
    mockAuth.AssertExpectations(t)
}
```

## Running Tests

### Unit Tests (No Docker Required)

```bash
# Run only unit tests (fast, skips integration tests)
make test-unit
# or
go test -short ./...

# Run specific unit test
go test -short -run TestSpecificFunction ./internal/api
```

### Integration Tests (Docker Required)

#### Standard Docker Setup
```bash
# Run all tests (unit + integration)
make test

# Run only integration tests  
make test-integration

# Run with verbose output
make test-verbose
```

#### Colima Setup (macOS)
If you're using Colima instead of Docker Desktop:

```bash
# Run all tests with Colima
make test-colima

# Run specific integration test with Colima
export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock" && \
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="/var/run/docker.sock" && \
go test -run TestLoginUser ./internal/api
```

### Docker Requirements

Integration tests require Docker for testcontainers:

#### Option 1: Docker Desktop
- **macOS**: Docker Desktop
- **Linux**: Docker daemon running
- **Windows**: Docker Desktop
- **Command**: `make test-integration`

#### Option 2: Colima (macOS Alternative)
- **Install**: `brew install colima`
- **Start**: `colima start`
- **Command**: `make test-colima`

#### Option 3: Manual Docker Setup
For custom Docker configurations:
```bash
export DOCKER_HOST="your-docker-host"
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="/var/run/docker.sock"
go test ./...
```

## Test Structure Best Practices

### Test Organization

```go
func TestFeatureName(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Common setup
    testDB := testutil.NewTestDatabase(t)
    testDB.RunMigrations(t)
    server := NewServer(testDB, mockJWT, mockAuth)

    t.Run("success case", func(t *testing.T) {
        // Test implementation
    })

    t.Run("error case", func(t *testing.T) {
        // Error test implementation
    })
}
```

### Table-Driven Tests

For testing multiple scenarios:

```go
func TestItemValidation(t *testing.T) {
    testDB := testutil.NewTestDatabase(t)
    testDB.RunMigrations(t)
    server := NewServer(testDB, nil, nil)

    tests := []struct {
        name        string
        request     api.CreateItemJSONRequestBody
        expectError bool
        errorType   interface{}
    }{
        {
            name: "valid item",
            request: api.CreateItemJSONRequestBody{
                Name:  "Valid Item",
                Type:  api.ItemTypeLow,
                Stock: 5,
            },
            expectError: false,
        },
        {
            name: "empty name",
            request: api.CreateItemJSONRequestBody{
                Name:  "",
                Type:  api.ItemTypeLow,
                Stock: 5,
            },
            expectError: true,
            errorType:   api.CreateItem400JSONResponse{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Test Data Isolation

Each test should create its own data:

```go
func TestMultipleUsers(t *testing.T) {
    testDB := testutil.NewTestDatabase(t)
    testDB.RunMigrations(t)

    // Each subtest creates its own users
    t.Run("admin operations", func(t *testing.T) {
        adminUser := testDB.NewUser(t).AsGlobalAdmin().Create()
        // Test with admin...
    })

    t.Run("member operations", func(t *testing.T) {
        memberUser := testDB.NewUser(t).AsMember().Create()
        // Test with member...
    })
}
```

## Common Testing Patterns

### Testing Request Validation

```go
func TestRequestValidation(t *testing.T) {
    server := NewServer(testDB, nil, nil)
    ctx := authenticatedContext(testUser)

    t.Run("missing required field", func(t *testing.T) {
        request := api.CreateItemRequestObject{
            Body: &api.CreateItemJSONRequestBody{
                // Missing required "name" field
                Type:  api.ItemTypeLow,
                Stock: 1,
            },
        }

        response, err := server.CreateItem(ctx, request)
        
        require.NoError(t, err)
        require.IsType(t, api.CreateItem400JSONResponse{}, response)
    })
}
```

### Testing Database Interactions

```go
func TestDatabaseOperations(t *testing.T) {
    testDB := testutil.NewTestDatabase(t)
    testDB.RunMigrations(t)

    // Create test data
    itemID := testDB.NewItem(t).WithName("Test Item").Create()

    // Test retrieving the item
    ctx := context.Background()
    item, err := testDB.Queries().GetItemByID(ctx, itemID)
    
    require.NoError(t, err)
    assert.Equal(t, "Test Item", item.Name)
}
```

### Testing Error Scenarios

```go
func TestErrorHandling(t *testing.T) {
    testDB := testutil.NewTestDatabase(t)
    testDB.RunMigrations(t)
    
    mockAuth := testutil.NewMockAuthenticator(t)
    server := NewServer(testDB, nil, mockAuth)

    testUser := testDB.NewUser(t).AsMember().Create()

    t.Run("database error", func(t *testing.T) {
        // Simulate database error by using invalid ID
        mockAuth.ExpectCheckPermission(testUser.ID, "view_items", nil, true, nil)

        ctx := authenticatedContext(testUser)
        response, err := server.GetItemById(ctx, api.GetItemByIdRequestObject{
            Id: uuid.New(), // Non-existent ID
        })

        require.NoError(t, err)
        require.IsType(t, api.GetItemById404JSONResponse{}, response)
    })
}
```

## Performance Testing

For load testing individual handlers:

```go
func BenchmarkGetItems(b *testing.B) {
    testDB := testutil.NewTestDatabase(&testing.T{}) // Note: not ideal for benchmarks
    testDB.RunMigrations(&testing.T{})
    
    // Create test data
    for i := 0; i < 100; i++ {
        testDB.NewItem(&testing.T{}).Create()
    }

    server := NewServer(testDB, nil, mockAuth)
    ctx := authenticatedContext(testUser)
    request := api.GetItemsRequestObject{}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := server.GetItems(ctx, request)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## Helper Functions

Common helper functions for tests:

```go
// Helper to create authenticated context
func authenticatedContext(user *testutil.TestUser) context.Context {
    return context.WithValue(context.Background(), auth.UserClaimsKey, &auth.AuthenticatedUser{
        ID:    user.ID,
        Email: user.Email,
    })
}

// Helper for string pointers
func stringPtr(s string) *string {
    return &s
}

// Helper for int pointers  
func intPtr(i int) *int {
    return &i
}
```

This testing approach provides comprehensive coverage while maintaining fast execution and reliable test isolation.