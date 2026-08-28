#!/usr/bin/env bash
# wait-for-it.sh - block until a TCP host:port is reachable, then exec a command.
# Usage: wait-for-it.sh host:port [host:port ...] [-t timeout] [-- command args]
set -euo pipefail

TIMEOUT=60
TARGETS=()
CMD=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -t) TIMEOUT="$2"; shift 2 ;;
    --) shift; CMD=("$@"); break ;;
    *) TARGETS+=("$1"); shift ;;
  esac
done

wait_one() {
  local host="${1%%:*}" port="${1##*:}" start now
  start=$(date +%s)
  until (exec 3<>"/dev/tcp/${host}/${port}") 2>/dev/null; do
    now=$(date +%s)
    if (( now - start >= TIMEOUT )); then
      echo "wait-for-it: timeout waiting for ${host}:${port}" >&2
      return 1
    fi
    sleep 1
  done
  exec 3>&- 2>/dev/null || true
  echo "wait-for-it: ${host}:${port} is up"
}

for t in "${TARGETS[@]}"; do
  wait_one "$t"
done

if [[ ${#CMD[@]} -gt 0 ]]; then
  exec "${CMD[@]}"
fi
