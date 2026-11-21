#!/usr/bin/env bash
# Deploy a release built by GitHub Actions: pull images → backup → migrations → start → health check.
# On a failed start or health check the API and web go back to the previous release automatically.
# Usage: scripts/deploy.sh <commit-sha>        (called by the workflow over SSH; can be run by hand)
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
tag="${1:?usage: deploy.sh <commit-sha>}"
[[ "$tag" =~ ^[0-9a-f]{40}$ ]] || die "not a full commit sha: $tag"
require_env_file
cd "$ROOT"
exec 9> "$ROOT/.deploy.lock"
flock -n 9 || die "another deploy is running"

prev="$(cat "$ROOT/current_tag" 2>/dev/null || true)"
export IMAGE_TAG="$tag"
log "release $tag (previous: ${prev:-none})"

log "pulling images"
"$DC" pull --quiet

log "starting postgres"
"$DC" up -d --wait --wait-timeout 120 postgres

if [[ -n "$prev" ]]; then
  log "backup before migrations"
  IMAGE_TAG="$prev" "$ROOT/scripts/backup.sh" "pre-${tag:0:12}"
fi

log "migrations"
"$DC" run --rm migrate || die "migrations failed; the previous release keeps running (backup: backups/*-pre-${tag:0:12}*)"

rollback() {
  [[ -n "$prev" ]] || die "$1; no previous release to return to"
  log "$1 — returning api and web to $prev"
  IMAGE_TAG="$prev" "$DC" up -d --no-deps --wait --wait-timeout 180 api web || true
  die "release $tag failed; $prev restored. Migrations of $tag stay applied — if $prev cannot work with them, use scripts/restore.sh with backups/db-*-pre-${tag:0:12}.dump"
}

log "starting api, web, caddy"
# --no-deps: postgres is up and migrations are applied above; the one-shot migrate job is not started again
# (older Compose versions also fail `up --wait` on a container that exits, even with code 0).
"$DC" up -d --no-deps --wait --wait-timeout 180 --remove-orphans api web caddy \
  || rollback "containers did not become healthy"
# The Caddyfile is a bind mount: apply a changed file without restarting (open WebSockets survive).
"$DC" exec -T caddy caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile \
  || rollback "Caddyfile reload failed (the previous Caddy config stays active)"

printf '%s\n' "$tag" > "$ROOT/current_tag"
"$ROOT/scripts/healthcheck.sh" || { printf '%s\n' "$prev" > "$ROOT/current_tag"; rollback "health check failed"; }
printf '%s %s deploy\n' "$(date -u +%FT%TZ)" "$tag" >> "$ROOT/releases.log"
# Only dangling layers; tagged images of earlier releases stay for rollback.
docker image prune -f > /dev/null
log "release $tag is live"
