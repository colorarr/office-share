#!/usr/bin/env bash
# Single-instance SQLite deployment. Files are immutable after upload.
set -euo pipefail
umask 077
DATA_DIR=${DATA_DIR:-/var/lib/office-share}
BACKUP_DIR=${BACKUP_DIR:-/var/backups/office-share}
mkdir -p "$BACKUP_DIR"
exec 9>"$BACKUP_DIR/.backup.lock"
flock -n 9 || exit 0
work=$(mktemp -d "$BACKUP_DIR/.snapshot.XXXXXX")
trap 'rm -rf "$work"' EXIT
python3 - "$DATA_DIR/office.db" "$work/office.db" <<'PY'
import sqlite3, sys
with sqlite3.connect(sys.argv[1]) as source, sqlite3.connect(sys.argv[2]) as target:
    source.backup(target)
    assert target.execute('PRAGMA integrity_check').fetchone()[0] == 'ok'
PY
archive="$BACKUP_DIR/office-share-$(date -u +%Y%m%dT%H%M%SZ).tar.gz"
tar -czf "$archive" -C "$work" office.db -C "$DATA_DIR" files
chmod 600 "$archive"
printf 'Backup created: %s\n' "$archive"
# No automatic deletion: set a retention policy appropriate to your workspace.
