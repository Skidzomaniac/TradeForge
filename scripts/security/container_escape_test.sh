#!/usr/bin/env bash
# Verifies contestant container isolation. Requires docker + a test image.
set -uo pipefail
IMG=${IMG:-trade-eval-contestant:test}
echo "== no internet egress on isolated network =="
docker run --rm --network contestant-isolated "$IMG" sh -c \
  'wget -T 5 -q -O- http://example.com >/dev/null 2>&1 && echo OPEN || echo BLOCKED' || echo BLOCKED
echo "== read-only rootfs =="
docker run --rm --read-only --network contestant-isolated "$IMG" sh -c \
  'echo x > /etc/test 2>&1 && echo WRITABLE || echo READONLY' || echo READONLY
