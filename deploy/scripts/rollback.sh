#!/usr/bin/env bash
# Roll the API and web back to an earlier release (images only). Migrations are NOT reverted:
# if the release being undone added migrations that the old code cannot work with, restore the
# database from the "pre-<sha>" backup taken by deploy.sh (scripts/restore.sh).
# Usage: scripts/rollback.sh [commit-sha]   (default: the previous release from releases.log)
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
require_env_file
cur="$(cat "$ROOT/current_tag" 2>/dev/null || true)"
tag="${1:-}"
if [[ -z "$tag" ]]; then
  tag="$(awk -v cur="$cur" '$2 != cur {t=$2} END {print t}' "$ROOT/releases.log" 2>/dev/null || true)"
  [[ -n "$tag" ]] || die "no previous release in releases.log; pass a commit sha"
fi
[[ "$tag" =~ ^[0-9a-f]{40}$ ]] || die "not a full commit sha: $tag"
[[ "$tag" != "$cur" ]] || die "$tag is already the current release"

export IMAGE_TAG="$tag"
reg="$(envget IMAGE_REGISTRY)"; reg="${reg:-ghcr.io/almukhanbetov/ortalyq-new}"
for img in api web; do
  docker image inspect "$reg/$img:$tag" > /dev/null 2>&1 || "$DC" pull "$img" \
    || die "image $reg/$img:$tag is not on the server and cannot be pulled (docker login ghcr.io first)"
done
log "switching api and web to $tag (was ${cur:-none})"
# --no-deps: the migrate job of the old image is not run against the newer schema.
"$DC" up -d --no-deps --wait --wait-timeout 180 api web
printf '%s\n' "$tag" > "$ROOT/current_tag"
printf '%s %s rollback\n' "$(date -u +%FT%TZ)" "$tag" >> "$ROOT/releases.log"
"$ROOT/scripts/healthcheck.sh"
