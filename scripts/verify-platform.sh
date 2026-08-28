#!/usr/bin/env bash
# Platform readiness verification. Green=pass, red=fail.
set -uo pipefail
BASE_URL=${BASE_URL:-http://localhost:8080}
LB_URL=${LB_URL:-http://localhost:8084}
G='\033[32m'; R='\033[31m'; N='\033[0m'
fail=0
ok()   { echo -e "${G}PASS${N} $1"; }
bad()  { echo -e "${R}FAIL${N} $1"; fail=1; }

curl -sf "$BASE_URL/v1/health" >/dev/null && ok "submission-api health" || bad "submission-api health"
curl -sf "$LB_URL/v1/health"   >/dev/null && ok "leaderboard-api health" || bad "leaderboard-api health"
curl -sf "$BASE_URL/v1/leaderboard" >/dev/null && ok "leaderboard endpoint" || bad "leaderboard endpoint"

[ $fail -eq 0 ] && echo -e "${G}PLATFORM READY${N}" || echo -e "${R}PLATFORM NOT READY${N}"
exit $fail
