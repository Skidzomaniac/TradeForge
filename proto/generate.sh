#!/usr/bin/env bash
# Regenerates Go code for all .proto files into proto/gen/go/.
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc on PATH.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="${SCRIPT_DIR}/gen/go"

mkdir -p "${OUT_DIR}"

protoc \
  --proto_path="${SCRIPT_DIR}" \
  --go_out="${OUT_DIR}" \
  --go_opt=paths=source_relative \
  --go-grpc_out="${OUT_DIR}" \
  --go-grpc_opt=paths=source_relative \
  "${SCRIPT_DIR}"/submission.proto \
  "${SCRIPT_DIR}"/test_events.proto \
  "${SCRIPT_DIR}"/telemetry.proto \
  "${SCRIPT_DIR}"/leaderboard.proto

echo "Generated Go protobuf code in ${OUT_DIR}"
