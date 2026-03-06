# Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o bin/server ./cmd/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/seeder ./scripts/seeder/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/goose github.com/pressly/goose/v3/cmd/goose

# Final stage
FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache bash

COPY --from=builder /app/bin/server    ./bin/server
COPY --from=builder /app/bin/seeder    ./bin/seeder
COPY --from=builder /app/bin/goose     ./bin/goose

COPY db/migrations/  ./db/migrations/
COPY templates/      ./templates/
COPY config/         ./config/

RUN mkdir -p /app/logs

COPY scripts/docker-entrypoint.sh ./entrypoint.sh
RUN sed -i 's/\r//' ./entrypoint.sh && chmod +x ./entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
