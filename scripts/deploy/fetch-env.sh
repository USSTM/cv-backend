#!/bin/bash
set -e

cd "$(dirname "$0")/../.."

echo "POSTGRES_PASSWORD=$(aws ssm get-parameter --region ca-central-1 --name /cv-backend/postgres-password --with-decryption --query Parameter.Value --output text)" > .env
echo "JWT_SIGNING_KEY=$(aws ssm get-parameter --region ca-central-1 --name /cv-backend/jwt-signing-key --with-decryption --query Parameter.Value --output text)" >> .env

chmod 600 .env
echo "Wrote .env from SSM Parameter Store."
