#!/bin/sh
set -eu
umask 077

env_file=${1:-}
archive=${2:-}
if [ -z "$env_file" ] || [ ! -f "$env_file" ] || [ -z "$archive" ] || [ ! -s "$archive" ]; then
  echo "usage: $0 /absolute/path/to/backup.env /absolute/path/to/suppq-backup.tar.age" >&2
  exit 2
fi
set -a
. "$env_file"
set +a
: "${SUPPQ_RESTORE_DATABASE_URL:?missing SUPPQ_RESTORE_DATABASE_URL}"
: "${SUPPQ_RESTORE_OBJECT_BUCKET:?missing SUPPQ_RESTORE_OBJECT_BUCKET}"
: "${SUPPQ_BACKUP_AGE_IDENTITY:?missing SUPPQ_BACKUP_AGE_IDENTITY}"
: "${SUPPQ_OBJECT_ENDPOINT_URL:?missing SUPPQ_OBJECT_ENDPOINT_URL}"
: "${SUPPQ_OBJECT_ACCESS_KEY:?missing SUPPQ_OBJECT_ACCESS_KEY}"
: "${SUPPQ_OBJECT_SECRET_KEY:?missing SUPPQ_OBJECT_SECRET_KEY}"
case "$SUPPQ_RESTORE_DATABASE_URL:$SUPPQ_RESTORE_OBJECT_BUCKET" in
  *drill*) ;;
  *) echo "restore targets must contain the word drill" >&2; exit 2 ;;
esac
command -v pg_restore >/dev/null
command -v psql >/dev/null
command -v mc >/dev/null
command -v age >/dev/null

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/suppq-restore.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT INT TERM
age -d -i "$SUPPQ_BACKUP_AGE_IDENTITY" "$archive" | tar -C "$work_dir" -xf -
(cd "$work_dir" && shasum -a 256 -c MANIFEST.sha256)
table_count=$(psql "$SUPPQ_RESTORE_DATABASE_URL" -Atc "select count(*) from information_schema.tables where table_schema='public' and table_type='BASE TABLE'")
if [ "$table_count" != "0" ]; then
  echo "restore drill database is not empty" >&2
  exit 2
fi
pg_restore --no-owner --no-privileges --dbname "$SUPPQ_RESTORE_DATABASE_URL" "$work_dir/database.dump"
mc --config-dir "$work_dir/mc" alias set target "$SUPPQ_OBJECT_ENDPOINT_URL" "$SUPPQ_OBJECT_ACCESS_KEY" "$SUPPQ_OBJECT_SECRET_KEY" >/dev/null
existing=$(mc --config-dir "$work_dir/mc" find "target/$SUPPQ_RESTORE_OBJECT_BUCKET" --maxdepth 1 2>/dev/null | wc -l | tr -d ' ')
if [ "$existing" != "0" ]; then
  echo "restore drill object bucket is not empty" >&2
  exit 2
fi
mc --config-dir "$work_dir/mc" mirror "$work_dir/objects" "target/$SUPPQ_RESTORE_OBJECT_BUCKET" >/dev/null
restored_tables=$(psql "$SUPPQ_RESTORE_DATABASE_URL" -Atc "select count(*) from information_schema.tables where table_schema='public' and table_type='BASE TABLE'")
restored_objects=$(mc --config-dir "$work_dir/mc" find "target/$SUPPQ_RESTORE_OBJECT_BUCKET" --type f | wc -l | tr -d ' ')
echo "restore drill passed: tables=$restored_tables objects=$restored_objects"
