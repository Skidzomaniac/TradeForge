#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost:8080}

echo "=== Multi-Contestant Platform Test ==="

upload_and_start() {
  local name=$1
  local key=$2
  local file=$3

  echo "==> [$name] Uploading $file..." >&2
  RESULT=$(curl -sf -X POST "$BASE_URL/v1/submissions" -H "X-API-Key: $key" -F "file=@$file" -F "language=cpp")
  SUB_ID=$(echo "$RESULT" | jq -r .submission_id)

  echo "==> [$name] Waiting for build..." >&2
  for _ in $(seq 1 30); do
    STATUS=$(curl -sf "$BASE_URL/v1/submissions/$SUB_ID" -H "X-API-Key: $key" | jq -r .status)
    [ "$STATUS" = "ready" ] && break
    [ "$STATUS" = "failed" ] && { echo "[$name] BUILD FAILED" >&2; return 1; }
    sleep 2
  done

  echo "==> [$name] Starting test..." >&2
  TEST_RESULT=$(curl -sf -X POST "$BASE_URL/v1/tests" -H "X-API-Key: $key" -H "Content-Type: application/json" -d "{\"submission_id\": \"$SUB_ID\", \"duration_seconds\": 30, \"bot_count\": 5}")
  TEST_ID=$(echo "$TEST_RESULT" | jq -r .test_id)
  echo "$TEST_ID"
}

echo "1. Deploying all bots..."
TEST_ALICE=$(upload_and_start "Alice (Fast)" "key-alice-0001" "testdata/fast-orderbook.cpp")
TEST_BOB=$(upload_and_start "Bob (Slow)" "key-bob-0002" "testdata/slow-orderbook.cpp")
TEST_CAROL=$(upload_and_start "Carol (Buggy)" "key-carol-0003" "testdata/buggy-orderbook.cpp")

echo "2. Waiting for tests to complete (up to 90s)..."
for _ in $(seq 1 45); do
  S_ALICE=$(curl -sf "$BASE_URL/v1/tests/$TEST_ALICE" -H "X-API-Key: key-alice-0001" | jq -r .test.status)
  S_BOB=$(curl -sf "$BASE_URL/v1/tests/$TEST_BOB" -H "X-API-Key: key-bob-0002" | jq -r .test.status)
  S_CAROL=$(curl -sf "$BASE_URL/v1/tests/$TEST_CAROL" -H "X-API-Key: key-carol-0003" | jq -r .test.status)
  
  echo "  Status -> Alice: $S_ALICE | Bob: $S_BOB | Carol: $S_CAROL"
  if [ "$S_ALICE" = "completed" ] && [ "$S_BOB" = "completed" ] && [ "$S_CAROL" = "completed" ]; then
    break
  fi
  sleep 2
done

echo "3. Final Leaderboard Rankings:"
curl -sf "$BASE_URL/v1/leaderboard" | jq .

echo "=== MULTI-TEST FINISHED ==="
