#!/bin/sh
set -eu
umask 077

env_file=${1:-}
if [ -z "$env_file" ] || [ ! -f "$env_file" ]; then
  echo "usage: $0 /absolute/path/to/backup.env" >&2
  exit 2
fi
set -a
. "$env_file"
set +a
: "${SUPPQ_DATABASE_URL:?missing SUPPQ_DATABASE_URL}"
: "${SUPPQ_OBJECT_ENDPOINT_URL:?missing SUPPQ_OBJECT_ENDPOINT_URL}"
: "${SUPPQ_OBJECT_ACCESS_KEY:?missing SUPPQ_OBJECT_ACCESS_KEY}"
: "${SUPPQ_OBJECT_SECRET_KEY:?missing SUPPQ_OBJECT_SECRET_KEY}"
: "${SUPPQ_OBJECT_BUCKET:?missing SUPPQ_OBJECT_BUCKET}"
: "${SUPPQ_BACKUP_DIR:?missing SUPPQ_BACKUP_DIR}"
: "${SUPPQ_BACKUP_AGE_RECIPIENT:?missing SUPPQ_BACKUP_AGE_RECIPIENT}"
command -v pg_dump >/dev/null
command -v mc >/dev/null
command -v age >/dev/null

mkdir -p "$SUPPQ_BACKUP_DIR"
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/suppq-backup.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT INT TERM
stamp=$(date -u +%Y%m%dT%H%M%SZ)
pg_dump --format=custom --no-owner --no-privileges --file "$work_dir/database.dump" "$SUPPQ_DATABASE_URL"
mc --config-dir "$work_dir/mc" alias set source "$SUPPQ_OBJECT_ENDPOINT_URL" "$SUPPQ_OBJECT_ACCESS_KEY" "$SUPPQ_OBJECT_SECRET_KEY" >/dev/null
mkdir -p "$work_dir/objects"
mc --config-dir "$work_dir/mc" mirror --overwrite "source/$SUPPQ_OBJECT_BUCKET" "$work_dir/objects" >/dev/null
(cd "$work_dir" && find database.dump objects -type f -print0 | sort -z | xargs -0 shasum -a 256 > MANIFEST.sha256)
archive="$SUPPQ_BACKUP_DIR/suppq-$stamp.tar.age"
tar -C "$work_dir" -cf - database.dump objects MANIFEST.sha256 | age -r "$SUPPQ_BACKUP_AGE_RECIPIENT" -o "$archive"
test -s "$archive"
echo "$archive"
