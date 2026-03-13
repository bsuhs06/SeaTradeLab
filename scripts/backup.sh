#!/bin/bash
# Backup the AIS database to a compressed SQL dump.
# Usage: ./scripts/backup.sh [backup_dir]
#
# Runs pg_dump via docker exec against the ais-postgres container by default.
# If pg_dump is available locally, set USE_LOCAL=1 to use it directly.
#
# Environment:
#   CONTAINER     — Docker container name (default: ais-postgres)
#   PGUSER        — Database user (required)
#   PGDATABASE    — Database name (default: ais_data)
#   KEEP_BACKUPS  — number of backups to retain (default: 10)
#   USE_LOCAL     — set to 1 to use local pg_dump instead of docker exec

set -euo pipefail

BACKUP_DIR="${1:-$(dirname "$0")/../backups}"
KEEP_BACKUPS="${KEEP_BACKUPS:-10}"

CONTAINER="${CONTAINER:-ais-postgres}"
PGUSER="${PGUSER:?Set PGUSER environment variable}"
PGDATABASE="${PGDATABASE:-ais_data}"
USE_LOCAL="${USE_LOCAL:-0}"

# Ensure backup directory exists
mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/ais_data_${TIMESTAMP}.sql.gz"

echo "=== AIS Database Backup ==="
echo "Target: ${BACKUP_FILE}"

if [ "$USE_LOCAL" = "1" ]; then
    # Local mode: use pg_dump directly
    PGHOST="${PGHOST:-localhost}"
    PGPORT="${PGPORT:-5432}"
    export PGPASSWORD="${PGPASSWORD:?Set PGPASSWORD environment variable}"

    echo "Mode:   local (pg_dump → ${PGHOST}:${PGPORT})"

    if ! command -v pg_dump >/dev/null 2>&1; then
        echo "ERROR: pg_dump not found. Install postgresql-client or use docker mode (default)." >&2
        exit 1
    fi

    if ! pg_isready -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" -q 2>/dev/null; then
        echo "ERROR: Database is not reachable at ${PGHOST}:${PGPORT}" >&2
        exit 1
    fi

    if pg_dump -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDATABASE" \
        --no-owner --no-privileges --clean --if-exists \
        | gzip > "$BACKUP_FILE"; then
        SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
        echo "OK: Backup complete (${SIZE})"
    else
        echo "ERROR: pg_dump failed" >&2
        rm -f "$BACKUP_FILE"
        exit 1
    fi
else
    # Docker mode: use docker exec (default)
    echo "Mode:   docker exec → ${CONTAINER}"

    if ! docker ps --filter name="$CONTAINER" --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
        echo "ERROR: Container '${CONTAINER}' is not running." >&2
        echo "Start it with: docker compose up -d postgres" >&2
        exit 1
    fi

    if docker exec "$CONTAINER" pg_dump -U "$PGUSER" -d "$PGDATABASE" \
        --no-owner --no-privileges --clean --if-exists \
        | gzip > "$BACKUP_FILE"; then
        SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
        echo "OK: Backup complete (${SIZE})"
    else
        echo "ERROR: pg_dump failed" >&2
        rm -f "$BACKUP_FILE"
        exit 1
    fi
fi

# Rotate old backups — keep the most recent $KEEP_BACKUPS
BACKUP_COUNT=$(find "$BACKUP_DIR" -name 'ais_data_*.sql.gz' -type f | wc -l | tr -d ' ')
if [ "$BACKUP_COUNT" -gt "$KEEP_BACKUPS" ]; then
    REMOVE_COUNT=$((BACKUP_COUNT - KEEP_BACKUPS))
    echo "Rotating: removing ${REMOVE_COUNT} old backup(s), keeping ${KEEP_BACKUPS}"
    find "$BACKUP_DIR" -name 'ais_data_*.sql.gz' -type f \
        | sort | head -n "$REMOVE_COUNT" \
        | while read -r old; do
            rm -f "$old"
            echo "  Removed: $(basename "$old")"
        done
fi

echo "Backups on disk: $(find "$BACKUP_DIR" -name 'ais_data_*.sql.gz' -type f | wc -l | tr -d ' ')"
echo "=== Done ==="
