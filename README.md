# Campus Vault Backend

Inventory management system for USSTM (Undergraduate Science Society of Toronto Metropolitan University).

## Quick Start

```bash
# 1. Setup environment
cp .env.sample .env

# 2. Start database
docker-compose up -d

# 3. Run migrations & seed data
make migrate-up
make seed

# 4. Build & run
make run
```

## Common Commands

```bash
make build              # Build the application
make run                # Build and run
make generate           # Generate code from OpenAPI/SQL
make migrate-up         # Apply database migrations
make migrate-down       # Rollback last migration
make db-reset           # Fresh database (wipes all data)
make seed               # Load seed data
make reseed             # Nuke and reseed
make test               # Run all tests
```

## Tech Stack

- Go 1.24+
- PostgreSQL
- OpenAPI (oapi-codegen)
- Docker

## Project Structure

```
├── api/                # OpenAPI specs
├── cmd/                # Entry points (main, seeder)
├── config/             # Seed data configs
├── db/
│   ├── migrations/    # Database migrations
│   └── queries/       # SQL queries (sqlc)
├── generated/         # Auto-generated code
└── internal/          # Application code
```

## Seeder Tool

Seed the database with test data from YAML files:

```bash
# Seed from single file
export $(cat .env | xargs) && go run cmd/seeder/main.go seed --file config/dev-seed.yaml

# Seed from directory (combines all .yaml files)
export $(cat .env | xargs) && go run cmd/seeder/main.go seed --dir config/frontend-test

# Validate without seeding
export $(cat .env | xargs) && go run cmd/seeder/main.go seed --file config/dev-seed.yaml --dry-run

# Nuke database (rollback migrations, wipe data)
export $(cat .env | xargs) && go run cmd/seeder/main.go nuke --force
```

Or use make targets (handles env automatically): `make seed`, `make nuke`, `make reseed`

## Contributing

1. Branch from `main` using `feature/name` or `bugfix/name`
2. Write tests
3. Format code with `gofmt`
4. Submit PR

See `docs/` for detailed guides.
