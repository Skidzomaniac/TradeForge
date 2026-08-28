#!/usr/bin/env bash
# End-to-end integration test against a running full stack. Requires curl + jq.
set -euo pipefail
BASE_URL=${BASE_URL:-http://localhost:8080}
API_KEY=${API_KEY:-key-alice-0001}

# Dump service logs to help debug a timeout, then exit non-zero.
fail() {
  echo "$1"
  if command -v docker >/dev/null 2>&1; then
    for svc in submission-api build-worker orchestrator bot-fleet telemetry-ingester leaderboard-api; do
      echo "===== logs: $svc ====="
      docker compose logs --no-color "$svc" 2>/dev/null | tail -30
    done
  fi
  exit 1
}

echo "== creating submission =="
RESULT=$(curl -sf -X POST "$BASE_URL/v1/submissions" \
  -H "X-API-Key: $API_KEY" \
  -F "file=@testdata/sample-orderbook.zip" \
  -F "language=cpp")
SUB_ID=$(echo "$RESULT" | jq -r .submission_id)
echo "submission: $SUB_ID"

# Building pulls the contestant base image on a cold runner, so allow 240s.
echo "== waiting for READY (240s) =="
STATUS=""
for _ in $(seq 1 120); do
  STATUS=$(curl -sf "$BASE_URL/v1/submissions/$SUB_ID" -H "X-API-Key: $API_KEY" | jq -r .status)
  echo "  $STATUS"
  [ "$STATUS" = "ready" ] && break
  [ "$STATUS" = "failed" ] && fail "build failed"
  sleep 2
done
[ "$STATUS" = "ready" ] || fail "submission never became ready (last status: $STATUS)"

echo "== triggering test =="
TEST_ID=$(curl -sf -X POST "$BASE_URL/v1/tests" \
  -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" \
  -d "{\"submission_id\":\"$SUB_ID\",\"duration_seconds\":30,\"bot_count\":10}" | jq -r .test_id)
echo "test: $TEST_ID"

echo "== waiting for completion (180s) =="
STATUS=""
for _ in $(seq 1 90); do
  STATUS=$(curl -sf "$BASE_URL/v1/tests/$TEST_ID" -H "X-API-Key: $API_KEY" | jq -r .test.status)
  echo "  $STATUS"
  [ "$STATUS" = "completed" ] && break
  [ "$STATUS" = "failed" ] && fail "test failed"
  sleep 2
done
[ "$STATUS" = "completed" ] || fail "test never completed (last status: $STATUS)"

echo "== leaderboard =="
LB=$(curl -sf "$BASE_URL/v1/leaderboard")
echo "$LB" | jq .
COUNT=$(echo "$LB" | jq '.entries | length')
[ "$COUNT" -ge 1 ] || fail "no leaderboard entries"
SCORE=$(echo "$LB" | jq '.entries[0].score')
echo "top score: $SCORE"
echo "INTEGRATION TEST PASSED"
