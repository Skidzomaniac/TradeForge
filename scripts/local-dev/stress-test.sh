#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:8080}
API_KEY=${API_KEY:-key-alice-0001}

echo "=== Platform Stress Test ==="

echo "==> Uploading testdata/fast-orderbook.cpp..."
RESULT=$(curl -sf -X POST "$BASE_URL/v1/submissions" \
  -H "X-API-Key: $API_KEY" \
  -F "file=@testdata/fast-orderbook.cpp" \
  -F "language=cpp")
SUB_ID=$(echo "$RESULT" | jq -r .submission_id)
echo "Submission ID: $SUB_ID"

echo "==> Waiting for build (up to 120s)..."
for _ in $(seq 1 60); do
  STATUS=$(curl -sf "$BASE_URL/v1/submissions/$SUB_ID" -H "X-API-Key: $API_KEY" | jq -r .status)
  [ "$STATUS" = "ready" ] && break
  [ "$STATUS" = "failed" ] && { echo "BUILD FAILED"; exit 1; }
  sleep 2
done

echo "==> Triggering stress test (bot_count=1000, duration=60s)..."
TEST_ID=$(curl -sf -X POST "$BASE_URL/v1/tests" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"submission_id\":\"$SUB_ID\",\"duration_seconds\":60,\"bot_count\":1000}" | jq -r .test_id)
echo "Test ID: $TEST_ID"

echo "==> Waiting for test completion..."
for _ in $(seq 1 90); do
  STATUS=$(curl -sf "$BASE_URL/v1/tests/$TEST_ID" -H "X-API-Key: $API_KEY" | jq -r .test.status)
  echo "  Status: $STATUS"
  [ "$STATUS" = "completed" ] && break
  [ "$STATUS" = "failed" ] && { echo "TEST FAILED"; exit 1; }
  sleep 2
done

echo "==> Fetching final leaderboard..."
LB=$(curl -sf "$BASE_URL/v1/leaderboard")
echo "$LB" | jq .

echo "=== STRESS TEST COMPLETED ==="
