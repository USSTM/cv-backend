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
  if [ -n "${SEED_DIR}" ]; then
    /app/bin/seeder seed --dir "${SEED_DIR}" --skip-if-seeded
  else
    SEED_FILE="${SEED_FILE:-/app/config/dev-seed.yaml}"
    /app/bin/seeder seed --file "${SEED_FILE}" --skip-if-seeded
  fi
fi

echo "Starting server..."
exec /app/bin/server
