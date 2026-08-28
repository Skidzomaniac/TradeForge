package anomaly

import (
	"math"
	"sync"

	"github.com/trade-eval/telemetry-ingester/model"
)

// ZScoreDetector tracks a running mean/variance (Welford) of latency per
// contestant and flags samples that deviate beyond a sigma threshold.
type ZScoreDetector struct {
	mu    sync.Mutex
	state map[string]*welford
}

type welford struct {
	count int64
	mean  float64
	m2    float64
}

// NewZScoreDetector constructs the detector.
func NewZScoreDetector() *ZScoreDetector {
	return &ZScoreDetector{state: make(map[string]*welford)}
}

// Observe updates the running stats and returns the z-score of value.
func (d *ZScoreDetector) Observe(contestantID string, value float64) float64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	w := d.state[contestantID]
	if w == nil {
		w = &welford{}
		d.state[contestantID] = w
	}
	w.count++
	delta := value - w.mean
	w.mean += delta / float64(w.count)
	w.m2 += delta * (value - w.mean)
	if w.count < 2 {
		return 0
	}
	std := math.Sqrt(w.m2 / float64(w.count-1))
	if std == 0 {
		return 0
	}
	return (value - w.mean) / std
}

// IsAnomaly reports whether value is beyond threshold sigma, after a 100-sample
// warm-up.
func (d *ZScoreDetector) IsAnomaly(contestantID string, value, threshold float64) bool {
	d.mu.Lock()
	w := d.state[contestantID]
	warm := w != nil && w.count > 100
	d.mu.Unlock()
	if !warm {
		d.Observe(contestantID, value)
		return false
	}
	z := d.Observe(contestantID, value)
	return math.Abs(z) > threshold
}

// Classify assigns a coarse behaviour label from a snapshot's shape.
func Classify(p50, p99 int64, correctness float64) string {
	switch {
	case p99 < 10 && correctness >= 1.0:
		return "CACHING"
	case p99 > 0 && p50 > 0 && float64(p99)/float64(p50) > 10:
		return "INCONSISTENT"
	case correctness >= 0.999 && p99 > 0 && p99 < 1000:
		return "CONSISTENT_HIGH_PERFORMER"
	default:
		return "NORMAL"
	}
}

// MLInspect augments rule-based detection with z-score latency outliers.
func (d *Detector) MLInspect(zdet *ZScoreDetector, e model.TelemetryEvent) []Report {
	if e.TimedOut || e.LatencyUs <= 0 {
		return nil
	}
	if zdet.IsAnomaly(e.ContestantID, float64(e.LatencyUs), 4.0) {
		return []Report{{
			ContestantID: e.ContestantID,
			Type:         "LATENCY_OUTLIER",
			Severity:     "INFO",
			Details:      "latency > 4 sigma from this contestant's own baseline",
		}}
	}
	return nil
}
