#!/bin/bash
# Restore the AIS database from a backup file.
# Usage: ./scripts/restore.sh <backup_file>
#
# Runs psql via docker exec against the ais-postgres container by default.
# If psql is available locally, set USE_LOCAL=1 to use it directly.
#
# Accepts .sql.gz (compressed) or .sql (plain) files.
#
# Environment:
#   CONTAINER  — Docker container name (default: ais-postgres)
#   PGUSER     — Database user (required)
#   PGDATABASE — Database name (default: ais_data)
#   USE_LOCAL  — set to 1 to use local psql instead of docker exec

set -euo pipefail

BACKUP_FILE="${1:?Usage: $0 <backup_file.sql.gz|backup_file.sql>}"

if [ ! -f "$BACKUP_FILE" ]; then
    echo "ERROR: File not found: ${BACKUP_FILE}" >&2
    exit 1
fi

CONTAINER="${CONTAINER:-ais-postgres}"
PGUSER="${PGUSER:?Set PGUSER environment variable}"
PGDATABASE="${PGDATABASE:-ais_data}"
USE_LOCAL="${USE_LOCAL:-0}"

echo "=== AIS Database Restore ==="
echo "Source: ${BACKUP_FILE}"

if [ "$USE_LOCAL" = "1" ]; then
    PGHOST="${PGHOST:-localhost}"
    PGPORT="${PGPORT:-5432}"
    export PGPASSWORD="${PGPASSWORD:?Set PGPASSWORD environment variable}"
    echo "Target: ${PGUSER}@${PGHOST}:${PGPORT}/${PGDATABASE}"
else
    echo "Target: docker exec → ${CONTAINER} (${PGUSER}@${PGDATABASE})"
fi

echo ""
echo "WARNING: This will overwrite the current database contents."
printf "Continue? [y/N] "
read -r CONFIRM
if [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ]; then
    echo "Aborted."
    exit 0
fi

if [ "$USE_LOCAL" = "1" ]; then
    if ! pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" -q 2>/dev/null; then
        echo "ERROR: Database is not reachable at ${PGHOST}:${PGPORT}" >&2
        exit 1
    fi

    echo "Restoring..."
    case "$BACKUP_FILE" in
        *.sql.gz)
            gunzip -c "$BACKUP_FILE" | psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" \
                -v ON_ERROR_STOP=0 --quiet
            ;;
        *.sql)
            psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" \
                -v ON_ERROR_STOP=0 --quiet -f "$BACKUP_FILE"
            ;;
        *)
            echo "ERROR: Unsupported file format. Expected .sql or .sql.gz" >&2
            exit 1
            ;;
    esac
else
    if ! docker ps --filter name="$CONTAINER" --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
        echo "ERROR: Container '${CONTAINER}' is not running." >&2
        echo "Start it with: docker compose up -d postgres" >&2
        exit 1
    fi

    echo "Restoring..."
    case "$BACKUP_FILE" in
        *.sql.gz)
            gunzip -c "$BACKUP_FILE" | docker exec -i "$CONTAINER" psql -U "$PGUSER" -d "$PGDATABASE" \
                -v ON_ERROR_STOP=0 --quiet
            ;;
        *.sql)
            docker exec -i "$CONTAINER" psql -U "$PGUSER" -d "$PGDATABASE" \
                -v ON_ERROR_STOP=0 --quiet < "$BACKUP_FILE"
            ;;
        *)
            echo "ERROR: Unsupported file format. Expected .sql or .sql.gz" >&2
            exit 1
            ;;
    esac
fi

echo "OK: Restore complete."
echo "=== Done ==="
