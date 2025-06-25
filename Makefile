.PHONY: generate-api generate-db generate build run clean

# Generate API boilerplate from OpenAPI spec
generate-api:
	oapi-codegen --config=api/config.yaml api/swagger.yaml

# Generate database code from SQL
generate-db:
	sqlc generate

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
	rm -rf generated/