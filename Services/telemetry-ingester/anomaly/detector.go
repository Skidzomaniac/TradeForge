// Package anomaly flags suspicious contestant performance.
package anomaly

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/trade-eval/telemetry-ingester/metrics"
)

// Report describes one detected anomaly.
type Report struct {
	ContestantID string    `json:"contestant_id"`
	Type         string    `json:"type"`
	Severity     string    `json:"severity"`
	Details      string    `json:"details"`
	DetectedAt   time.Time `json:"detected_at"`
}

// Detector inspects snapshots for suspicious patterns.
type Detector struct {
	redis *redis.Client
}

// NewDetector constructs the detector.
func NewDetector(rdb *redis.Client) *Detector { return &Detector{redis: rdb} }

// Inspect returns any anomalies for a snapshot.
func (d *Detector) Inspect(s metrics.Snapshot) []Report {
	var reports []Report
	now := time.Now()

	if s.P99LatencyUs < 1 && s.TotalOrders > 100 {
		reports = append(reports, Report{s.ContestantID, "LATENCY_TOO_LOW", "WARNING",
			"p99 < 1µs with >100 orders suggests no real processing", now})
	}
	if s.CorrectnessRate >= 1.0 && s.P99LatencyUs < 10 && s.TotalOrders > 100 {
		reports = append(reports, Report{s.ContestantID, "PERFECT_CORRECTNESS_WITH_HIGH_SPEED", "INFO",
			"perfect correctness with sub-10µs p99 - review for precomputed responses", now})
	}
	return reports
}

// Publish stores anomalies in Redis (capped list of 100).
func (d *Detector) Publish(ctx context.Context, reports []Report) {
	if d.redis == nil {
		return
	}
	for _, r := range reports {
		if b, err := json.Marshal(r); err == nil {
			key := "anomalies:" + r.ContestantID
			d.redis.LPush(ctx, key, b)
			d.redis.LTrim(ctx, key, 0, 99)
		}
	}
}
