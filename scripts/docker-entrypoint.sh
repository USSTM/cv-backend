#!/bin/bash
set -e

echo "Waiting for PostgreSQL..."
until bash -c ">/dev/tcp/db/5432" 2>/dev/null; do
  sleep 1
done

echo "Running database migrations..."
/app/bin/goose -dir /app/db/migrations up

if [ "${SEED_ON_START}" = "true" ]; then
  echo "Seeding database..."
  /app/bin/seeder seed --file /app/config/dev-seed.yaml
fi

echo "Starting server..."
exec /app/bin/server
