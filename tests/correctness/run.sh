#!/usr/bin/env bash
# Correctness suite for any order book at $ORDER_BOOK_URL (default :8080).
set -uo pipefail
U=${ORDER_BOOK_URL:-http://localhost:8080}
fail=0
expect() { echo "$1" | grep -q "$2" && echo "PASS: $3" || { echo "FAIL: $3 -> $1"; fail=1; }; }

curl -sf -XPOST "$U/reset" >/dev/null 2>&1 || true

# basic fill
curl -sf -XPOST "$U/order" -d '{"order_id":"s1","type":"LIMIT_SELL","price":100,"quantity":10}' >/dev/null
R=$(curl -sf -XPOST "$U/order" -d '{"order_id":"b1","type":"LIMIT_BUY","price":100,"quantity":10}')
expect "$R" "FILLED" "limit cross fills"

# resting limit
curl -sf -XPOST "$U/reset" >/dev/null 2>&1 || true
R=$(curl -sf -XPOST "$U/order" -d '{"order_id":"b2","type":"LIMIT_BUY","price":50,"quantity":5}')
expect "$R" "PENDING" "non-crossing limit rests"

# cancel
R=$(curl -sf -XPOST "$U/cancel" -d '{"order_id":"b2"}')
expect "$R" "CANCELLED" "cancel resting order"

[ $fail -eq 0 ] && echo "ALL CORRECTNESS TESTS PASSED" || echo "SOME TESTS FAILED"
exit $fail
