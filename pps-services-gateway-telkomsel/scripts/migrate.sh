#!/bin/sh
# Database migration script for CI/CD pipeline.
# Usage: ./scripts/migrate.sh [up|down|version]
#
# Requires POSTGRES_DSN environment variable.
# In Docker: migrate binary is pre-installed at /usr/local/bin/migrate
# Migration files are at /app/migrations

set -e

MIGRATIONS_PATH="${MIGRATIONS_PATH:-/app/migrations}"
ACTION="${1:-up}"

if [ -z "$POSTGRES_DSN" ]; then
    echo "ERROR: POSTGRES_DSN environment variable is required"
    exit 1
fi

echo "Running migration: ${ACTION}"
echo "Migrations path: ${MIGRATIONS_PATH}"

migrate -path "$MIGRATIONS_PATH" -database "$POSTGRES_DSN" "$ACTION"

echo "Migration completed successfully"
