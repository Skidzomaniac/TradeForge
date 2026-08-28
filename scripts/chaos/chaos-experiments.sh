#!/usr/bin/env bash
# Chaos experiments against a running k8s/compose stack.
set -uo pipefail
exp=${1:-help}
case "$exp" in
  kafka_broker_death)
    docker restart trade-eval-kafka; sleep 30
    docker exec trade-eval-kafka kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --group telemetry-ingesters || true
    ;;
  orchestrator_crash_during_test)
    docker restart trade-eval-orchestrator; sleep 70
    echo "orchestrator restarted; orphan detection should recover running tests within 60s"
    ;;
  all)
    "$0" kafka_broker_death; "$0" orchestrator_crash_during_test ;;
  *)
    echo "usage: $0 {kafka_broker_death|orchestrator_crash_during_test|all}" ;;
esac
