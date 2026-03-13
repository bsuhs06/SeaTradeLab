#!/bin/sh
# Run SQL migrations against the database.
# Usage: ./run_migrations.sh <database_url> [migrations_dir]
#
# Requires: psql (PostgreSQL client)
#
# This applies each .up.sql file in order.

set -e

DATABASE_URL="${1:?Usage: $0 <database_url> [migrations_dir]}"
MIGRATIONS_DIR="${2:-$(dirname "$0")/../migrations}"

if ! command -v psql >/dev/null 2>&1; then
  echo "Error: psql not found. Install postgresql-client." >&2
  exit 1
fi

echo "Applying migrations from $MIGRATIONS_DIR ..."

for f in "$MIGRATIONS_DIR"/*.up.sql; do
  [ -f "$f" ] || continue
  echo "  -> $(basename "$f")"
  psql "$DATABASE_URL" -f "$f" -v ON_ERROR_STOP=1
done

echo "Migrations complete."
