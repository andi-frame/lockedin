#!/bin/sh
# Look at, test and restore the Postgres backups in the Garage bucket (docs/RUNBOOK.md).
#   restore.sh list             what is in the bucket
#   restore.sh check <key>      restore the backup into a scratch database, count what is in it, drop it.
#                               Touches nothing live: this is the restore drill.
#   restore.sh restore <key>    REPLACES the live database with the backup. Stop the api and the worker
#                               first. Needs CONFIRM_RESTORE=tepati. A copy of the live database is put
#                               in the bucket under pre-restore/ before anything is replaced.
# <key> looks like daily/2026-10-09.dump, weekly/2026-W41.dump or (the safety copy a restore leaves)
# pre-restore/20261009T091451Z.dump.
set -eu

: "${DATABASE_URL:?DATABASE_URL is not set}"
: "${S3_ENDPOINT:?S3_ENDPOINT is not set}"
: "${S3_ACCESS_KEY:?S3_ACCESS_KEY is not set}"
: "${S3_SECRET_KEY:?S3_SECRET_KEY is not set}"
BUCKET="${S3_BUCKET_BACKUPS:-tepati-backups}"
SCRATCH=tepati_restore_check

export AWS_ACCESS_KEY_ID="$S3_ACCESS_KEY" AWS_SECRET_ACCESS_KEY="$S3_SECRET_KEY" AWS_DEFAULT_REGION="${S3_REGION:-garage}"
export AWS_REQUEST_CHECKSUM_CALCULATION=when_required AWS_RESPONSE_CHECKSUM_VALIDATION=when_required

s3() { aws --endpoint-url "$S3_ENDPOINT" "$@"; }
log() { echo "$(date -u +%FT%TZ) restore: $*"; }
die() { log "$*" >&2; exit 1; }
# The connection string with another database name (the part between the last / and the ?).
url_for() { echo "$DATABASE_URL" | sed -E "s#^(postgres(ql)?://[^/]+)/[^?]*#\1/$1#"; }

action="${1:-}"
key="${2:-}"

case "$action" in
  list)
    for folder in daily weekly pre-restore; do
      echo "== $folder/"
      s3 s3 ls "s3://$BUCKET/$folder/" || echo "(nothing here, or the listing failed: read any error above)"
    done
    exit 0
    ;;
  check | restore) ;;
  *) die "usage: restore.sh list | check <key> | restore <key>" ;;
esac

case "$key" in
  daily/????-??-??.dump | weekly/????-W??.dump | pre-restore/????????T??????Z.dump) ;;
  *) die "give the backup's key, for example daily/2026-10-09.dump" ;;
esac

file="$(mktemp)"
s3 s3 cp "s3://$BUCKET/$key" "$file" --only-show-errors || die "could not download $key"
[ -s "$file" ] || die "$key is empty"
pg_restore --list "$file" > /dev/null || die "$key is not a readable pg_dump archive"
log "downloaded $key ($(wc -c < "$file") bytes), the archive is readable"

if [ "$action" = check ]; then
  admin="$(url_for postgres)"
  scratch="$(url_for "$SCRATCH")"
  psql "$admin" -v ON_ERROR_STOP=1 -qc "DROP DATABASE IF EXISTS $SCRATCH" -c "CREATE DATABASE $SCRATCH" || die "could not create the scratch database"
  pg_restore --no-owner --exit-on-error -d "$scratch" "$file" || die "pg_restore failed: this backup does not restore"
  log "restored into $SCRATCH; what it holds:"
  psql "$scratch" -At -F ' ' -c "select 'migration', max(version_id) from goose_db_version" \
    -c "select 'users', count(*) from users" -c "select 'pacts', count(*) from pacts" -c "select 'check_ins', count(*) from check_ins" \
    -c "select 'proofs', count(*) from proofs" -c "select 'ledger_entries', count(*) from ledger_entries" -c "select 'decisions', count(*) from decisions" \
    -c "select 'pots_that_do_not_add_up', count(*) from (select pact_id from ledger_entries group by pact_id having sum(amount) < 0) t"
  psql "$admin" -v ON_ERROR_STOP=1 -qc "DROP DATABASE $SCRATCH"
  log "scratch database dropped; the live database was not touched. $key is a good backup."
  exit 0
fi

# restore: the live database
[ "${CONFIRM_RESTORE:-}" = tepati ] || die "restore replaces the live database; set CONFIRM_RESTORE=tepati (bun run restore ... --live does)"
safety="$(mktemp)"
pg_dump --format=custom --no-owner "$DATABASE_URL" > "$safety" || die "could not take a safety copy of the live database, nothing was replaced"
[ -s "$safety" ] || die "the safety copy is empty, nothing was replaced"
safety_key="pre-restore/$(date -u +%Y%m%dT%H%M%SZ).dump"
s3 s3 cp "$safety" "s3://$BUCKET/$safety_key" --only-show-errors || die "could not store the safety copy, nothing was replaced"
log "safety copy of the live database stored as $safety_key"
pg_restore --clean --if-exists --no-owner --exit-on-error -d "$DATABASE_URL" "$file" || die "pg_restore failed part-way: the live database may be incomplete; put $safety_key back with this same script"
log "the live database now holds $key. Start the api and the worker."
