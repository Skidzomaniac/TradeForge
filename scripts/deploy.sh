#!/usr/bin/env bash
set -euo pipefail
ENV=${ENV:-production}
helm dependency update infra/helm/trade-eval-platform || true
helm upgrade --install trade-eval-platform infra/helm/trade-eval-platform \
  --namespace trade-eval --create-namespace \
  --values infra/helm/trade-eval-platform/values.yaml \
  --set global.imageTag="$(git rev-parse --short HEAD)" \
  --wait --timeout 10m
