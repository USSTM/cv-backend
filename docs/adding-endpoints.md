# Adding New Endpoints

This guide shows how to add new API endpoints to the project using the StrictServerInterface pattern.

## Step 1: Define in OpenAPI Spec

Add your endpoint to `api/swagger.yaml`. Reference the [OpenAPI 3.0 Specification](https://swagger.io/specification/) for syntax.

```yaml
paths:
  /my-endpoint:
    get:
      tags:
        - MyFeature
      summary: Description of what this does
      description: Longer description
      operationId: getMyEndpoint
      security:
        - BearerAuth: []
        - OAuth2: [required_permission_name]
      responses:
        "200":
          description: Success response
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
        "400":
          description: Bad request
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
        "401":
          description: Unauthorized
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
        "403":
          description: Forbidden
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
        "500":
          description: Internal server error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
```

**Important**: Always include 400, 401, 403, and 500 error responses to enable proper type generation for StrictServerInterface.

## Step 2: Add Database Queries (if needed)

If your endpoint needs database access, add queries to appropriate file in `db/queries/`. Follow [sqlc documentation](https://docs.sqlc.dev/en/latest/tutorials/getting-started.html) for query syntax.

```sql
-- name: GetMyData :many
SELECT id, name, created_at FROM my_table WHERE user_id = $1;

-- name: CreateMyRecord :one
INSERT INTO my_table (name, user_id) VALUES ($1, $2) RETURNING id;
```

Learn more about sqlc query annotations: [sqlc Query Annotations](https://docs.sqlc.dev/en/latest/reference/query-annotations.html)

## Step 3: Generate Code

Run the code generation:

```bash
make generate
```

This updates:
- `generated/api/api.gen.go` - API types and interfaces (including StrictServerInterface)
- `generated/db/` - Database query functions

## Step 4: Implement Handler

Add your handler to the appropriate file in `internal/api/` (e.g., `internal/api/myfeature.go`):

```go
package api

import (
    "context"
    "log"

    "github.com/USSTM/cv-backend/generated/api"
    "github.com/USSTM/cv-backend/internal/auth"
)

// GetMyEndpoint implements the StrictServerInterface
func (s Server) GetMyEndpoint(ctx context.Context, request api.GetMyEndpointRequestObject) (api.GetMyEndpointResponseObject, error) {
    // Check authentication
    user, ok := auth.GetAuthenticatedUser(ctx)
    if !ok {
        return api.GetMyEndpoint401JSONResponse{
            Code:    401,
            Message: "Unauthorized",
        }, nil
    }
    
    // Check permissions (see permissions.md for details)
    hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "required_permission_name", nil)
    if err != nil {
        log.Printf("Error checking permission: %v", err)
        return api.GetMyEndpoint500JSONResponse{
            Code:    500,
            Message: "Internal server error",
        }, nil
    }
    if !hasPermission {
        return api.GetMyEndpoint403JSONResponse{
            Code:    403,
            Message: "Insufficient permissions",
        }, nil
    }
    
    // Your business logic here
    data, err := s.db.Queries().GetMyData(ctx, user.ID)
    if err != nil {
        log.Printf("Database error: %v", err)
        return api.GetMyEndpoint500JSONResponse{
            Code:    500,
            Message: "Internal server error",
        }, nil
    }
    
    // Return success response
    return api.GetMyEndpoint200JSONResponse{
        Message: "Success",
        Data:    data,
    }, nil
}
```

## Step 5: Build and Test

Build the application:

```bash
make build
```

Start the server:

```bash
make run
```

Test your endpoint:

```bash
# Login to get token
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin2@test.com"}' | jq -r '.token')

# Test your endpoint
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/my-endpoint
```

## StrictServerInterface Benefits

The StrictServerInterface pattern provides several advantages:

1. **Type Safety**: Request and response objects are strongly typed
2. **Automatic Validation**: Request validation happens automatically
3. **No HTTP Boilerplate**: No need to manually handle HTTP request/response parsing
4. **Better Testing**: Handlers can be tested directly without HTTP mocking

## Common Patterns

### GET endpoint with query parameters

```yaml
/items:
  get:
    parameters:
      - name: category
        in: query
        schema:
          type: string
      - name: limit
        in: query
        schema:
          type: integer
          default: 10
```

Handler:
```go
func (s Server) GetItems(ctx context.Context, request api.GetItemsRequestObject) (api.GetItemsResponseObject, error) {
    // Access query parameters via request.Params
    category := request.Params.Category  // *string (optional)
    limit := request.Params.Limit        // *int (optional)
    
    // Your logic here...
}
```

### POST endpoint with request body

```yaml
/items:
  post:
    requestBody:
      required: true
      content:
        application/json:
          schema:
            type: object
            properties:
              name:
                type: string
              description:
                type: string
            required:
              - name
    responses:
      "201":
        description: Created
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/Item"
```

Handler:
```go
func (s Server) CreateItem(ctx context.Context, request api.CreateItemRequestObject) (api.CreateItemResponseObject, error) {
    // Validate request body exists (automatic validation ensures required fields)
    if request.Body == nil {
        return api.CreateItem400JSONResponse{
            Code:    400,
            Message: "Request body is required",
        }, nil
    }
    
    req := *request.Body
    // req.Name is guaranteed to exist (required field)
    // req.Description is *string (optional field)
    
    // Your logic here...
    
    return api.CreateItem201JSONResponse{
        // Return created item
    }, nil
}
```

### Path parameters

```yaml
/items/{id}:
  get:
    parameters:
      - name: id
        in: path
        required: true
        schema:
          $ref: "#/components/schemas/UUID"
```

Handler:
```go
func (s Server) GetItemById(ctx context.Context, request api.GetItemByIdRequestObject) (api.GetItemByIdResponseObject, error) {
    // Access path parameter via request.Id
    itemID := request.Id  // api.UUID type
    
    // Your logic here...
}
```

## Testing Handlers

With StrictServerInterface, you can test handlers directly:

```go
func TestGetMyEndpoint(t *testing.T) {
    // Setup test database
    testDB := testutil.NewTestDatabase(t)
    testDB.RunMigrations(t)
    
    // Create test user
    testUser := testDB.NewUser(t).
        WithEmail("test@example.com").
        AsMember().
        Create()
    
    // Create server
    server := NewServer(testDB, mockJWT, mockAuth)
    
    // Create context with authenticated user
    ctx := context.WithValue(context.Background(), auth.UserClaimsKey, &auth.AuthenticatedUser{
        ID:    testUser.ID,
        Email: testUser.Email,
    })
    
    // Call handler directly
    response, err := server.GetMyEndpoint(ctx, api.GetMyEndpointRequestObject{})
    
    // Assert response
    require.NoError(t, err)
    require.IsType(t, api.GetMyEndpoint200JSONResponse{}, response)
}
```

For more testing examples, see the existing tests in `internal/api/auth_test.go`.

## Permission Management

For details on available permissions and how to manage them, see [Managing Permissions](./permissions.md).

To find current permissions, check:
- Database: `SELECT name, description FROM permissions;`
- Migration file: `db/migrations/20250626025312_seed_roles_permissions.sql`
- Code: Look at existing handlers for examples

## Error Handling Best Practices

1. **Always return appropriate error types**: Use the generated response types (e.g., `api.GetMyEndpoint400JSONResponse`)
2. **Log errors for debugging**: Use `log.Printf()` for server errors
3. **Don't expose internal errors**: Return generic messages to clients
4. **Use consistent error codes**: 400 for client errors, 401 for auth, 403 for permissions, 500 for server errors

## Migration from Old Pattern

If you're updating an existing handler from the old HTTP pattern to StrictServerInterface:

1. Change the function signature to match the generated interface
2. Remove HTTP request/response handling code
3. Access request data via the request object
4. Return typed response objects instead of writing to ResponseWriter
5. Remove manual JSON encoding/decoding
6. Update tests to call handlers directly

Example migration:
```go
// Old pattern
func (s Server) GetItems(w http.ResponseWriter, r *http.Request) {
    // Manual HTTP handling...
}

// New pattern  
func (s Server) GetItems(ctx context.Context, request api.GetItemsRequestObject) (api.GetItemsResponseObject, error) {
    // Type-safe, validated request handling...
}
```