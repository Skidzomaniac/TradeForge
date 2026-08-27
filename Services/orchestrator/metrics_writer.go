package main

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// MetricsWriter publishes live + final metrics to Redis for the leaderboard.
type MetricsWriter struct {
	redis *redis.Client
}

// NewMetricsWriter constructs the writer.
func NewMetricsWriter(rdb *redis.Client) *MetricsWriter { return &MetricsWriter{redis: rdb} }

// PublishLiveMetrics writes a rolling snapshot during a test.
func (mw *MetricsWriter) PublishLiveMetrics(ctx context.Context, contestantID, name string, w LatencyWindow) error {
	pipe := mw.redis.Pipeline()
	pipe.HSet(ctx, "metrics:"+contestantID,
		"p50_latency_us", w.P50Us,
		"p90_latency_us", w.P90Us,
		"p99_latency_us", w.P99Us,
		"tps", w.TPS,
		"correctness_rate", w.CorrectnessRate,
		"last_updated_ns", time.Now().UnixNano(),
		"contestant_name", name,
		"test_status", "running",
	)
	pipe.SAdd(ctx, "leaderboard:active_contestants", contestantID)
	_, err := pipe.Exec(ctx)
	return err
}

// PublishFinalScore writes the terminal snapshot with composite_score.
func (mw *MetricsWriter) PublishFinalScore(ctx context.Context, contestantID, name string, w LatencyWindow, score float64) error {
	pipe := mw.redis.Pipeline()
	pipe.HSet(ctx, "metrics:"+contestantID,
		"p50_latency_us", w.P50Us,
		"p90_latency_us", w.P90Us,
		"p99_latency_us", w.P99Us,
		"tps", w.TPS,
		"correctness_rate", w.CorrectnessRate,
		"composite_score", score,
		"last_updated_ns", time.Now().UnixNano(),
		"contestant_name", name,
		"test_status", "completed",
	)
	pipe.SAdd(ctx, "leaderboard:active_contestants", contestantID)
	_, err := pipe.Exec(ctx)
	return err
}

// ClearMetrics resets a contestant's metrics at the start of a new test.
func (mw *MetricsWriter) ClearMetrics(ctx context.Context, contestantID string) error {
	return mw.redis.Del(ctx, "metrics:"+contestantID).Err()
}

// readWindow loads the latest snapshot Redis holds for a contestant.
func (mw *MetricsWriter) readWindow(ctx context.Context, contestantID string) (LatencyWindow, string) {
	m, err := mw.redis.HGetAll(ctx, "metrics:"+contestantID).Result()
	if err != nil {
		return LatencyWindow{ContestantID: contestantID}, ""
	}
	return LatencyWindow{
		ContestantID:    contestantID,
		P50Us:           atoiI64(m["p50_latency_us"]),
		P90Us:           atoiI64(m["p90_latency_us"]),
		P99Us:           atoiI64(m["p99_latency_us"]),
		TPS:             atof(m["tps"]),
		CorrectnessRate: atof(m["correctness_rate"]),
	}, m["contestant_name"]
}
