#!/usr/bin/env bash
# Create all Kafka topics with production settings.
set -euo pipefail
B=${BOOTSTRAP:-localhost:9092}
create() { kafka-topics.sh --bootstrap-server "$B" --create --if-not-exists "$@"; }
create --topic submissions          --partitions 4  --replication-factor 3 --config retention.ms=604800000
create --topic build-jobs           --partitions 4  --replication-factor 3 --config retention.ms=3600000
create --topic orchestrator-events  --partitions 4  --replication-factor 3 --config retention.ms=86400000
create --topic bot-telemetry        --partitions 16 --replication-factor 3 --config retention.ms=3600000 --config compression.type=snappy
create --topic leaderboard-updates  --partitions 2  --replication-factor 3
create --topic telemetry-anomalies  --partitions 2  --replication-factor 3
kafka-topics.sh --bootstrap-server "$B" --list
