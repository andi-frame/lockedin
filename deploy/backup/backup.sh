#!/bin/sh
# Nightly Postgres backup into the Garage bucket (docs/RUNNING.md §6).
#   daily/YYYY-MM-DD.dump     kept BACKUP_KEEP_DAILY (14) newest
#   weekly/YYYY-Www.dump      written on Sundays, kept BACKUP_KEEP_WEEKLY (8) newest
# The dump is pg_dump's custom format (already compressed); restore it with pg_restore.
# BACKUP_ONCE=1 takes one backup now and exits (for a first run and for tests).
set -eu

: "${DATABASE_URL:?DATABASE_URL is not set}"
: "${S3_ENDPOINT:?S3_ENDPOINT is not set}"
: "${S3_ACCESS_KEY:?S3_ACCESS_KEY is not set}"
: "${S3_SECRET_KEY:?S3_SECRET_KEY is not set}"
BUCKET="${S3_BUCKET_BACKUPS:-tepati-backups}"
KEEP_DAILY="${BACKUP_KEEP_DAILY:-14}"
KEEP_WEEKLY="${BACKUP_KEEP_WEEKLY:-8}"
HOUR_UTC="${BACKUP_HOUR_UTC:-3}"

# A retention of 0 would prune today's backup along with the old ones.
case "$KEEP_DAILY$KEEP_WEEKLY" in
  *[!0-9]* | "") echo "BACKUP_KEEP_DAILY and BACKUP_KEEP_WEEKLY must be whole numbers" >&2; exit 2 ;;
esac
[ "$KEEP_DAILY" -ge 1 ] && [ "$KEEP_WEEKLY" -ge 1 ] || { echo "BACKUP_KEEP_DAILY and BACKUP_KEEP_WEEKLY must be at least 1" >&2; exit 2; }

export AWS_ACCESS_KEY_ID="$S3_ACCESS_KEY" AWS_SECRET_ACCESS_KEY="$S3_SECRET_KEY" AWS_DEFAULT_REGION="${S3_REGION:-garage}"
# Garage does not take the checksums newer SDKs add by default.
export AWS_REQUEST_CHECKSUM_CALCULATION=when_required AWS_RESPONSE_CHECKSUM_VALIDATION=when_required

s3() { aws --endpoint-url "$S3_ENDPOINT" "$@"; }
log() { echo "$(date -u +%FT%TZ) backup: $*"; }

# prune <folder> <keep>: delete all but the newest <keep> objects (names sort by date).
prune() {
  s3 s3 ls "s3://$BUCKET/$1/" | awk '{print $4}' | grep . | sort -r | tail -n +"$(($2 + 1))" | while read -r key; do
    s3 s3 rm "s3://$BUCKET/$1/$key" --only-show-errors
    log "pruned $1/$key"
  done
}

# Every step is checked by hand: the loop below calls this inside `||`, where `set -e` does not
# apply, and a half-written dump must never be uploaded as if it were a backup.
backup_once() {
  tmp="$(mktemp)" || return 1
  pg_dump --format=custom --no-owner "$DATABASE_URL" > "$tmp" || { rm -f "$tmp"; return 1; }
  [ -s "$tmp" ] || { rm -f "$tmp"; log "pg_dump wrote nothing"; return 1; }
  s3 s3 cp "$tmp" "s3://$BUCKET/daily/$(date -u +%F).dump" --only-show-errors || { rm -f "$tmp"; return 1; }
  log "wrote daily/$(date -u +%F).dump ($(wc -c < "$tmp") bytes)"
  if [ "$(date -u +%u)" = 7 ]; then
    s3 s3 cp "$tmp" "s3://$BUCKET/weekly/$(date -u +%G-W%V).dump" --only-show-errors || { rm -f "$tmp"; return 1; }
    log "wrote weekly/$(date -u +%G-W%V).dump"
  fi
  rm -f "$tmp"
  prune daily "$KEEP_DAILY" && prune weekly "$KEEP_WEEKLY"
}

if [ "${BACKUP_ONCE:-}" = 1 ]; then
  backup_once || { log "FAILED"; exit 1; }
  exit 0
fi

# Then once a night at HOUR_UTC. A failed night is logged and the next one still runs.
while true; do
  now=$(( $(date -u +%H) * 3600 + $(date -u +%M) * 60 + $(date -u +%S) ))
  wait=$(( (HOUR_UTC * 3600 - now + 86400) % 86400 ))
  [ "$wait" -lt 60 ] && wait=$((wait + 86400))
  log "next backup in ${wait}s"
  sleep "$wait"
  backup_once || log "FAILED, will try again tomorrow"
done
