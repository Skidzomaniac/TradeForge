#!/usr/bin/env bash
set -euo pipefail
make -C "$(dirname "$0")/.." orderbook
"$(dirname "$0")/../orderbook" & PID=$!
sleep 0.5
trap 'kill $PID 2>/dev/null' EXIT
U=http://localhost:8080
curl -sf $U/reset >/dev/null
curl -sf -XPOST $U/order -d '{"order_id":"s1","type":"LIMIT_SELL","price":100,"quantity":5}' >/dev/null
R=$(curl -sf -XPOST $U/order -d '{"order_id":"b1","type":"LIMIT_BUY","price":100,"quantity":5}')
echo "$R" | grep -q FILLED && echo "PASS basic fill" || { echo "FAIL: $R"; exit 1; }
C=$(curl -sf -XPOST $U/cancel -d '{"order_id":"missing"}')
echo "$C" | grep -q NOT_FOUND && echo "PASS cancel missing" || { echo "FAIL: $C"; exit 1; }
echo "ALL PASS"
