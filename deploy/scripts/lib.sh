# Shared helpers for the deploy scripts (sourced, not executed).
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck disable=SC2034 # used by the scripts that source this file
DC="$ROOT/scripts/dc"
log() { printf '[%s %s] %s\n' "$(basename "$0" .sh)" "$(date -u +%H:%M:%S)" "$*"; }
die() { log "ERROR: $*" >&2; exit 1; }
# envget KEY — value from .env without sourcing it (the file is data, not a script).
envget() { grep -E "^$1=" "$ROOT/.env" | tail -n 1 | cut -d= -f2- || true; }
require_env_file() {
  [[ -f "$ROOT/.env" ]] || die "$ROOT/.env is missing (copy .env.production.example and fill it in)"
  local mode; mode="$(stat -c %a "$ROOT/.env")"
  [[ "$mode" == 600 || "$mode" == 400 ]] || die "$ROOT/.env must be chmod 600 (now $mode)"
  grep -q '^POSTGRES_PASSWORD=change-me' "$ROOT/.env" && die "POSTGRES_PASSWORD in .env is still the example value"
  return 0
}
