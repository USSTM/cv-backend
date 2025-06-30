# Project Structure

## Key Directories

- `api/` - OpenAPI specifications
- `db/migrations/` - Database schema migrations  
- `db/queries/` - SQL query definitions
- `generated/` - Auto-generated code (don't edit)
- `internal/api/` - HTTP handlers
- `internal/auth/` - Authentication/authorization

## Build Commands

```bash
make generate    # Generate code from OpenAPI + SQL
make build      # Compile application
make run        # Build and run server
```

## Development Flow

1. Define API in `api/swagger.yaml`
2. Add database queries in `db/queries/`
3. Run `make generate`
4. Implement handlers in `internal/api/`
5. Test with `make run`