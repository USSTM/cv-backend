#!/bin/bash
set -e

cd "$(dirname "$0")/../.."

FILE="backup-$(date +%F).sql.gz"

docker compose -f docker-compose.prod.yml exec -T db pg_dump -U postgres campus_vault | gzip > "/tmp/$FILE"
aws s3 cp "/tmp/$FILE" "s3://cv-backend-prod-bucket/db-backups/$FILE"
rm "/tmp/$FILE"

echo "Backed up to s3://cv-backend-prod-bucket/db-backups/$FILE"
