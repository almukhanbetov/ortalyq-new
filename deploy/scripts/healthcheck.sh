#!/usr/bin/env bash
# End-to-end check of the running release through Caddy on this host:
# containers healthy, HTTPS works, /healthz sees the DB, the site and API answer, http→https and www→apex redirect.
# Requests go to 127.0.0.1 (--resolve), so the check does not depend on DNS or hairpin NAT.
# Retries for up to HEALTH_TIMEOUT seconds (the first start waits for Let's Encrypt certificates).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SITE_DOMAIN="$(envget SITE_DOMAIN)"; WWW_DOMAIN="$(envget WWW_DOMAIN)"
HTTP_PORT="$(envget HTTP_PORT)"; HTTP_PORT="${HTTP_PORT:-80}"
HTTPS_PORT="$(envget HTTPS_PORT)"; HTTPS_PORT="${HTTPS_PORT:-443}"
[[ -n "$SITE_DOMAIN" && -n "$WWW_DOMAIN" ]] || die "SITE_DOMAIN / WWW_DOMAIN are not set in .env"
TIMEOUT="${HEALTH_TIMEOUT:-180}"
# CURL_EXTRA: only for local checks with Caddy's internal CA (e.g. -k). Empty on the VPS.
read -r -a EXTRA <<< "${CURL_EXTRA:-}"
c() { curl -sS --max-time 10 --resolve "$SITE_DOMAIN:$HTTPS_PORT:127.0.0.1" --resolve "$WWW_DOMAIN:$HTTPS_PORT:127.0.0.1" \
  --resolve "$SITE_DOMAIN:$HTTP_PORT:127.0.0.1" "${EXTRA[@]}" "$@"; }
BASE="https://$SITE_DOMAIN:$HTTPS_PORT"

check() {
  local bad=0 s
  for s in postgres api web; do
    local h; h="$("$DC" ps --format '{{.Health}}' "$s" 2>/dev/null || true)"
    [[ "$h" == healthy ]] || { echo "  $s: ${h:-not running}"; bad=1; }
  done
  [[ "$("$DC" ps --format '{{.State}}' caddy 2>/dev/null)" == running ]] || { echo "  caddy: not running"; bad=1; }
  local body; body="$(c "$BASE/healthz" 2>&1 || true)"
  [[ "$body" == *'"status":"ok"'* ]] || { echo "  GET /healthz: $body"; bad=1; }
  local code
  code="$(c -o /dev/null -w '%{http_code}' "$BASE/login" 2>/dev/null || true)"; [[ "$code" == 200 ]] || { echo "  GET /login: $code"; bad=1; }
  code="$(c -o /dev/null -w '%{http_code}' "$BASE/api/auth/me" 2>/dev/null || true)"; [[ "$code" == 401 ]] || { echo "  GET /api/auth/me (no session): $code, want 401"; bad=1; }
  local loc
  loc="$(c -o /dev/null -w '%{redirect_url}' "http://$SITE_DOMAIN:$HTTP_PORT/login" 2>/dev/null || true)"
  [[ "$loc" == "https://$SITE_DOMAIN"* ]] || { echo "  http → https redirect: '$loc'"; bad=1; }
  loc="$(c -o /dev/null -w '%{redirect_url}' "https://$WWW_DOMAIN:$HTTPS_PORT/login" 2>/dev/null || true)"
  [[ "$loc" == "https://$SITE_DOMAIN/login" ]] || { echo "  www → apex redirect: '$loc'"; bad=1; }
  return $bad
}

deadline=$(( $(date +%s) + TIMEOUT ))
while true; do
  if out="$(check)"; then log "OK: $BASE, release ${IMAGE_TAG:-$(cat "$ROOT/current_tag" 2>/dev/null)}"; exit 0; fi
  if (( $(date +%s) >= deadline )); then printf '%s\n' "$out" >&2; die "health check failed after ${TIMEOUT}s"; fi
  sleep 5
done
