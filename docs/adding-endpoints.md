# Adding New Endpoints

This guide shows how to add new API endpoints to the project.

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
```

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
- `generated/api/api.gen.go` - API types and interfaces
- `generated/db/` - Database query functions

## Step 4: Implement Handler

Add your handler to `internal/api/server.go`:

```go
func (s Server) GetMyEndpoint(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Check authentication
    user, ok := auth.GetAuthenticatedUser(ctx)
    if !ok {
        resp := api.Error{Code: 401, Message: "Unauthorized"}
        w.WriteHeader(401)
        _ = json.NewEncoder(w).Encode(resp)
        return
    }
    
    // Check permissions (see permissions.md for details)
    hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "required_permission_name", nil)
    if err != nil {
        log.Printf("Error checking permission: %v", err)
        resp := api.Error{Code: 500, Message: "Internal server error"}
        w.WriteHeader(500)
        _ = json.NewEncoder(w).Encode(resp)
        return
    }
    if !hasPermission {
        resp := api.Error{Code: 403, Message: "Insufficient permissions"}
        w.WriteHeader(403)
        _ = json.NewEncoder(w).Encode(resp)
        return
    }
    
    // Your business logic here
    data, err := s.db.Queries().GetMyData(ctx, user.ID)
    if err != nil {
        log.Printf("Database error: %v", err)
        resp := api.Error{Code: 500, Message: "Internal server error"}
        w.WriteHeader(500)
        _ = json.NewEncoder(w).Encode(resp)
        return
    }
    
    // Return success response
    response := map[string]interface{}{
        "message": "Success",
        "data": data,
    }
    w.WriteHeader(200)
    _ = json.NewEncoder(w).Encode(response)
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

## Common Patterns

### GET endpoint with query parameters

Reference: [OpenAPI Parameters](https://swagger.io/docs/specification/describing-parameters/)

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

### POST endpoint with request body

Reference: [OpenAPI Request Body](https://swagger.io/docs/specification/describing-request-body/)

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

## Permission Management

For details on available permissions and how to manage them, see [Managing Permissions](./permissions.md).

To find current permissions, check:
- Database: `SELECT name, description FROM permissions;`
- Code: Look at existing handlers for examples