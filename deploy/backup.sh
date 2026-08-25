#!/bin/sh

set -eu

SCRIPT_DIR="$(
    CDPATH= cd -- "$(dirname -- "$0")" &&
    pwd
)"

PROJECT_DIR="$(
    dirname "$SCRIPT_DIR"
)"

BACKUP_DIR="${BACKUP_DIR:-$PROJECT_DIR/backups}"

RETENTION_DAYS="${RETENTION_DAYS:-7}"

TIMESTAMP="$(
    date -u +"%Y%m%dT%H%M%SZ"
)"

AUTH_BACKUP="$BACKUP_DIR/auth_${TIMESTAMP}.dump"

LIBRARY_BACKUP="$BACKUP_DIR/library_${TIMESTAMP}.dump"

AUTH_TMP="${AUTH_BACKUP}.tmp"

LIBRARY_TMP="${LIBRARY_BACKUP}.tmp"


cleanup() {
    rm -f \
        "$AUTH_TMP" \
        "$LIBRARY_TMP"
}

trap cleanup EXIT


mkdir -p "$BACKUP_DIR"

chmod 700 "$BACKUP_DIR"


echo "Backing up auth database..."

docker exec auth-postgres \
    pg_dump \
    -U auth_user \
    -d auth_db \
    -Fc \
    > "$AUTH_TMP"


if [ ! -s "$AUTH_TMP" ]; then
    echo "Auth backup is empty" >&2

    exit 1
fi


mv \
    "$AUTH_TMP" \
    "$AUTH_BACKUP"


echo "Backing up library database..."

docker exec library-postgres \
    pg_dump \
    -U library_user \
    -d library_db \
    -Fc \
    > "$LIBRARY_TMP"


if [ ! -s "$LIBRARY_TMP" ]; then
    echo "Library backup is empty" >&2

    exit 1
fi


mv \
    "$LIBRARY_TMP" \
    "$LIBRARY_BACKUP"


echo "Removing backups older than ${RETENTION_DAYS} days..."

find "$BACKUP_DIR" \
    -type f \
    -name '*.dump' \
    -mtime "+${RETENTION_DAYS}" \
    -delete


echo "Backup completed."

echo "Auth:    $AUTH_BACKUP"

echo "Library: $LIBRARY_BACKUP"