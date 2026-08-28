#!/usr/bin/env bash
# migrate.sh - apply SQL migrations to both databases in lexical order.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

TIMESCALE_DSN="${TIMESCALE_DSN:-postgres://postgres:postgres@localhost:5432/tradeeval?sslmode=disable}"
ORCH_DSN="${ORCHESTRATOR_DB_DSN:-postgres://postgres:postgres@localhost:5433/orchestrator?sslmode=disable}"

apply() {
  local dsn="$1" dir="$2"
  [[ -d "$dir" ]] || { echo "no migrations dir: $dir"; return 0; }
  for f in $(ls "$dir"/*.sql | sort); do
    echo "==> applying $(basename "$f") to ${dsn%%\?*}"
    psql "$dsn" -v ON_ERROR_STOP=1 -f "$f"
  done
}

echo "== TimescaleDB migrations =="
apply "$TIMESCALE_DSN" "$ROOT/services/telemetry-ingester/migrations"

echo "== Orchestrator Postgres migrations =="
apply "$ORCH_DSN" "$ROOT/services/orchestrator/migrations"

echo "migrations complete"
