#!/usr/bin/env bash
# Backup: PostgreSQL (pg_dump custom format, verified with pg_restore --list) and the uploads volume (tar.gz).
# Usage: scripts/backup.sh [label]      → backups/db-<UTC time>-<label>.dump, backups/uploads-<UTC time>-<label>.tar.gz
# Keeps the newest BACKUP_KEEP (14) of each kind. Copy backups off the server too (see docs/deploy.md).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_env_file
label="${1:-manual}"; [[ "$label" =~ ^[A-Za-z0-9._-]+$ ]] || die "label: letters, digits, . _ - only"
keep="${BACKUP_KEEP:-14}"
dir="$ROOT/backups"; mkdir -p "$dir"; chmod 700 "$dir"; umask 077
ts="$(date -u +%Y%m%dT%H%M%SZ)"
db="$dir/db-$ts-$label.dump"; up="$dir/uploads-$ts-$label.tar.gz"

log "database → $db"
"$DC" exec -T postgres pg_dump -U bilimway -d bilimway -Fc > "$db.part"
"$DC" exec -T postgres pg_restore --list < "$db.part" > /dev/null || die "dump is not readable by pg_restore"
mv "$db.part" "$db"

log "uploads → $up"
# The API image has the uploads volume mounted at /data/uploads; --no-deps: the running stack is not touched.
"$DC" run --rm --no-deps -T --entrypoint tar api -czf - -C /data/uploads . > "$up.part"
gzip -t "$up.part" || die "uploads archive is corrupt"
mv "$up.part" "$up"

for kind in "db-*.dump" "uploads-*.tar.gz"; do
  # shellcheck disable=SC2012,SC2086 # $kind is a glob on purpose; generated names have no spaces
  ls -1t "$dir"/$kind 2>/dev/null | tail -n +"$((keep + 1))" | xargs -r rm -f --
done
log "done: $(du -h "$db" | cut -f1) db, $(du -h "$up" | cut -f1) uploads"
