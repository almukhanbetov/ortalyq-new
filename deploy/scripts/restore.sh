#!/usr/bin/env bash
# Restore the database (and optionally uploads) from backups made by backup.sh. Destroys the current data:
# a safety backup "pre-restore" is taken first. The site is down (502) while it runs.
# Usage: scripts/restore.sh backups/db-....dump [backups/uploads-....tar.gz]
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_env_file
db="${1:?usage: restore.sh <db dump> [uploads tar.gz]}"; up="${2:-}"
[[ -f "$db" ]] || die "no such file: $db"
[[ -z "$up" || -f "$up" ]] || die "no such file: $up"
"$DC" exec -T postgres pg_restore --list < "$db" > /dev/null || die "$db is not a pg_dump custom-format file"
if [[ "${CONFIRM:-}" != RESTORE ]]; then
  read -r -p "Current database${up:+ and uploads} will be REPLACED. Type RESTORE to continue: " answer
  [[ "$answer" == RESTORE ]] || die "cancelled"
fi

log "safety backup of the current state"
"$ROOT/scripts/backup.sh" pre-restore

log "stopping api and web"
"$DC" stop api web

log "recreating database bilimway from $db"
"$DC" exec -T postgres dropdb -U bilimway --force --if-exists bilimway
"$DC" exec -T postgres createdb -U bilimway -O bilimway bilimway
"$DC" exec -T postgres pg_restore -U bilimway -d bilimway --no-owner --exit-on-error < "$db"

if [[ -n "$up" ]]; then
  log "replacing uploads from $up"
  "$DC" run --rm --no-deps -T --entrypoint sh api -c 'find /data/uploads -mindepth 1 -delete && tar -xzf - -C /data/uploads' < "$up"
fi

log "starting the current release (migrations run first)"
"$DC" run --rm migrate
"$DC" up -d --no-deps --wait --wait-timeout 180 api web
"$ROOT/scripts/healthcheck.sh"
