.PHONY: generate-api generate-db generate build run clean migrate-up migrate-down migrate-status migrate-create db-reset test test-unit test-integration test-colima test-verbose

# Generate API boilerplate from OpenAPI spec
generate-api:
	go tool oapi-codegen --config=api/config.yaml api/swagger.yaml

# Generate database code from SQL
generate-db:
	export $$(cat .env | xargs) && go tool sqlc generate

# Generate all code
generate: generate-db generate-api

# Build the application
build: generate
	go build -o bin/server cmd/main.go

# Run the application
run: build
	export $$(cat .env | xargs) && ./bin/server

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf generated/db/*
	rm -rf generated/api/*

# Database migration commands
migrate-up:
	export $$(cat .env | xargs) && go tool goose -dir db/migrations up

migrate-down:
	export $$(cat .env | xargs) && go tool goose -dir db/migrations down

migrate-status:
	export $$(cat .env | xargs) && go tool goose -dir db/migrations status

migrate-create:
	go tool goose -dir db/migrations create $(name) sql

# Reset database - stops containers and removes volume
db-reset:
	docker-compose down
	docker volume rm cv-backend_pgdata || true
	docker-compose up -d

# Test commands
# Run all tests
test: test-unit test-integration

# Run unit tests only (no testcontainers)
test-unit:
	go test -short ./...

# Run integration tests (standard Docker)
test-integration:
	go test ./...

# Run integration tests with Colima setup
test-colima:
	export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock" && \
	export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="/var/run/docker.sock" && \
	go test ./...

# Run tests with verbose output
test-verbose:
	go test -v ./...
