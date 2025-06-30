# Database Operations

## Migrations

**Create migration:**
```bash
make migrate-create name=migration_name
```

**Apply migrations:**
```bash
make migrate-up
```

**Revert last migration:**
```bash
make migrate-down
```

**Check status:**
```bash
make migrate-status
```

**Reset database:**
```bash
make db-reset  # Destroys all data
```

## Adding Queries

1. Add SQL queries to files in `db/queries/`
2. Use sqlc annotations: `:one`, `:many`, `:exec`
3. Reference: [sqlc documentation](https://docs.sqlc.dev/)

**Generate code:**
```bash
make generate
```

## Database Access

**Create user:**
```bash
./bin/hashgen email password "connection_string_from_env" role scope
```

**Direct database access:**
```bash
docker exec cv-backend-db-1 psql -U $POSTGRES_USER -d $POSTGRES_DB
```

## File Structure

- `db/migrations/` - Migration files
- `db/queries/auth.sql` - User/auth queries  
- `db/queries/items.sql` - Item queries
- `generated/db/` - Generated Go code
- `sqlc.yaml` - sqlc configuration