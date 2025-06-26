# Campus Vault Backend (cv-backend)

Backend service providing the business logic and API endpoints for the Campus Vault inventory management system.

## Overview

Campus Vault is an inventory management system developed for the Undergraduate Science Society of Toronto Metropolitan University (USSTM). This backend service enables:

- Registration for Faculty of Science (FOS) student groups
- Equipment borrowing management
- Disposable item tracking and distribution
- Inventory management for USSTM resources

## Technology Stack

- **Language**: Go (Golang)
- **Database**: PostgreSQL
- **API Generation**: OpenAPI with oapi-codegen
- **Deployment**: Docker (for development)

## Prerequisites

- Go 1.24+ (required for tools directive support)
- Docker and Docker Compose
- PostgreSQL client (optional, for direct database access)

## Getting Started

### Environment Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/usstm/cv-backend.git
   cd cv-backend
   ```

2. Create an environment file:
   ```bash
   cp .env.sample .env
   ```
   
3. Modify the `.env` file with your specific configuration values (including database credentials and goose migration settings)

### Running the Application

1. Start the PostgreSQL database:
   ```bash
   docker-compose up -d
   ```

2. Run database migrations:
   ```bash
   make migrate-up
   ```

3. Generate code and build:
   ```bash
   make build
   ```

4. Run the application:
   ```bash
   make run
   ```

## Development

### Code Generation

- **API Code**: `make generate-api` - Generates Go code from OpenAPI specification
- **Database Code**: `make generate-db` - Generates Go code from SQL queries using sqlc
- **All Code**: `make generate` - Runs both API and database code generation

### Database Migrations

We use [goose](https://github.com/pressly/goose) for database migrations:

- **Run migrations**: `make migrate-up` - Applies all pending migrations
- **Rollback**: `make migrate-down` - Rolls back the last migration
- **Status**: `make migrate-status` - Shows migration status
- **Create new migration**: `make migrate-create name=migration_name` - Creates a new migration file
- **Reset database**: `make db-reset` - Completely resets the database (removes volume and restarts)

Migration files are stored in `db/migrations/` and follow the goose format with `-- +goose Up` and `-- +goose Down` sections.

### Building and Running

- **Build**: `make build` - Builds the application
- **Run**: `make run` - Builds and runs the application
- **Clean**: `make clean` - Removes build artifacts

### API Development

This project uses OpenAPI specifications with oapi-codegen for automated API development:

1. API endpoints are defined in OpenAPI specification files in the project
2. The Go server code is auto-generated using oapi-codegen
3. We use Go 1.24's tools directive in go.mod for managing tool dependencies

### Tools Management

This project leverages Go 1.24's new tools directive system for managing development tools:

1. Tools are tracked directly in go.mod using `tool` directives
2. To add a new tool dependency:
   ```bash
   go get -tool github.com/deepmap/oapi-codegen/cmd/oapi-codegen
   ```

3. To run tools tracked in go.mod:
   ```bash
   go tool oapi-codegen [arguments]
   go tool sqlc [arguments]
   go tool goose [arguments]
   ```

4. To install all project tools to your GOBIN:
   ```bash
   go install tool
   ```

5. To update all tools:
   ```bash
   go get tool
   ```

## Project Structure

```
├── api/                    # OpenAPI specifications
├── cmd/                    # Application entry points
├── db/
│   ├── migrations/        # Database migration files (goose)
│   └── queries/           # SQL queries for sqlc
├── generated/             # Generated code (API and DB)
├── internal/              # Internal application code
├── docker-compose.yml     # Database setup
└── Makefile              # Build automation
```

## Database Schema

The database schema is managed through migrations in `db/migrations/`:

- `initial_schema.sql` - Creates all tables, enums, and constraints
- `seed_roles_permissions.sql` - Seeds default roles and permissions

The system includes:
- **User management**: Users, roles, permissions with scope-based access control
- **Inventory management**: Items with different types and approval workflows
- **Booking system**: Time slots, availability, and scheduling
- **Request/approval workflow**: For high-value items requiring approval

## Contributing

We welcome contributions from the USSTM and FOS community! Here's how to contribute:

### Branching Strategy

- Each feature should be developed in its own branch
- Branch naming convention: `feature/feature-name` or `bugfix/issue-description`
- Feature branches should be short-lived (1-2 weeks maximum)
- Related features can be grouped into a larger branch when necessary

### Development Workflow

1. Create a new branch from `main`
2. Implement your changes
3. Write tests for your code
4. Format your code using `gofmt`
5. Submit a pull request

### Code Standards

- All code must be formatted with `gofmt`
- Follow standard Go best practices and conventions
- Document public functions and packages
- Keep functions small and focused on a single responsibility
- Maintain OpenAPI specifications in sync with implementation
- Avoid modifying auto-generated code directly

### Testing

- Write unit tests for all new functionality
- Ensure all existing tests pass before submitting a PR
- Integration tests should be included for API endpoints

### Pull Requests

- PRs should address a specific feature or bug
- Include a clear description of the changes
- Reference any related issues
- PRs will be reviewed by at least one maintainer before merging

### Direct Commits

Direct commits to the `main` branch are restricted to:
- CI/CD configuration fixes
- Emergency patches (with approval)

## License

This project is licensed under the GNU General Public License v3.0 (GPL-3.0) - see the [LICENSE](LICENSE) file for details.
