#!/usr/bin/env bash
# Smoke test against a running stack. Requires curl + jq.
set -euo pipefail
BASE_URL=${BASE_URL:-http://localhost:8080}
API_KEY=${API_KEY:-key-alice-0001}

echo "=== Trade Eval Platform Smoke Test ==="

echo "==> Health check..."
curl -sf "$BASE_URL/v1/health" | jq .

echo "==> Uploading submission..."
FILE=${FILE:-testdata/sample-orderbook.zip}
if [ ! -f "$FILE" ]; then
  echo "  (no $FILE; using sample-orderbook.cpp)"
  FILE=testdata/sample-orderbook.cpp
fi
RESULT=$(curl -sf -X POST "$BASE_URL/v1/submissions" \
  -H "X-API-Key: $API_KEY" \
  -F "file=@$FILE" \
  -F "language=cpp")
echo "$RESULT" | jq .
SUB_ID=$(echo "$RESULT" | jq -r .submission_id)

echo "==> Waiting for build (60s timeout)..."
for _ in $(seq 1 30); do
  STATUS=$(curl -sf "$BASE_URL/v1/submissions/$SUB_ID" -H "X-API-Key: $API_KEY" | jq -r .status)
  echo "  Status: $STATUS"
  [ "$STATUS" = "ready" ] && break
  [ "$STATUS" = "failed" ] && { echo "BUILD FAILED"; exit 1; }
  sleep 2
done

echo "==> Starting test..."
TEST_RESULT=$(curl -sf -X POST "$BASE_URL/v1/tests" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"submission_id\": \"$SUB_ID\", \"duration_seconds\": 30, \"bot_count\": 10}")
echo "$TEST_RESULT" | jq .
TEST_ID=$(echo "$TEST_RESULT" | jq -r .test_id)

echo "==> Waiting for test completion (90s timeout)..."
for _ in $(seq 1 45); do
  STATUS=$(curl -sf "$BASE_URL/v1/tests/$TEST_ID" -H "X-API-Key: $API_KEY" | jq -r .test.status)
  echo "  Status: $STATUS"
  [ "$STATUS" = "completed" ] && break
  [ "$STATUS" = "failed" ] && { echo "TEST FAILED"; exit 1; }
  sleep 2
done

echo "==> Checking leaderboard..."
curl -sf "$BASE_URL/v1/leaderboard" | jq .

echo "=== SMOKE TEST PASSED ==="
