#!/usr/bin/env bash
set -euo pipefail
command -v k6 >/dev/null || { echo "k6 not installed; see https://k6.io"; exit 1; }
mkdir -p scripts/load-test/results/"$(date +%Y%m%d)"
k6 run scripts/load-test/submission_api_load.js
VUS=100 k6 run scripts/load-test/websocket_load.js
