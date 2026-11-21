#!/usr/bin/env bash
# Recreates the LOCAL demo database: drop schema, migrate, seed. Never use against production.
set -euo pipefail
cd "$(dirname "$0")/../backend"
set -a; . ./.env.example; set +a
case "$DATABASE_URL" in *localhost:5448*|*@postgres:5432*) ;; *) echo "refusing: not the local demo DB"; exit 1;; esac
docker compose -f ../docker-compose.yml exec -T postgres psql -q -U ortalyq_new -d ortalyq_new -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;" >/dev/null 2>&1
go run ./cmd/api migrate up
go run ./cmd/api seed
