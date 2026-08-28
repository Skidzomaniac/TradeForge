package storage

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/trade-eval/telemetry-ingester/metrics"
)

// RedisMetricsWriter publishes rolling metrics to Redis for the leaderboard.
// The ingester does not have access to the contestants table, so it writes
// metrics keyed by contestant id; the leaderboard uses the id as the display
// name.
type RedisMetricsWriter struct {
	redis *redis.Client
}

// NewRedisMetricsWriter constructs the writer.
func NewRedisMetricsWriter(rdb *redis.Client) *RedisMetricsWriter {
	return &RedisMetricsWriter{redis: rdb}
}

// PublishSnapshot writes a snapshot using a single pipelined round trip.
func (w *RedisMetricsWriter) PublishSnapshot(ctx context.Context, s metrics.Snapshot) error {
	if w.redis == nil {
		return nil
	}
	pipe := w.redis.Pipeline()
	pipe.HSet(ctx, "metrics:"+s.ContestantID,
		"p50_latency_us", s.P50LatencyUs,
		"p90_latency_us", s.P90LatencyUs,
		"p99_latency_us", s.P99LatencyUs,
		"tps", s.CurrentTPS,
		"peak_tps", s.PeakTPS,
		"correctness_rate", s.CorrectnessRate,
		"total_orders", s.TotalOrders,
		"valid_orders", s.ValidOrders,
		"correct_orders", s.CorrectOrders,
		"hard_violations", s.HardViolations,
		"last_updated_ns", time.Now().UnixNano(),
	)
	pipe.SAdd(ctx, "leaderboard:active_contestants", s.ContestantID)
	_, err := pipe.Exec(ctx)
	return err
}

// PublishBehavior records a contestant's behaviour classification.
func (w *RedisMetricsWriter) PublishBehavior(ctx context.Context, contestantID, behavior string) {
	if w.redis == nil {
		return
	}
	w.redis.HSet(ctx, "metrics:"+contestantID, "behavior", behavior)
}
