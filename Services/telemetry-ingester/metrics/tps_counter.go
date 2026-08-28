// Package metrics aggregates per-contestant performance.
package metrics

import (
	"sync"
	"time"
)

// tpsBucket holds the count for one second and the epoch second it represents.
// Tagging the bucket with its second lets a read ignore a bucket left over from
// an earlier minute, so an inactivity gap reads as zero rather than stale.
type tpsBucket struct {
	sec   int64
	count int64
}

// TPSCounter tracks orders-per-second per contestant using a ring of 60
// one-second buckets.
type TPSCounter struct {
	mu      sync.Mutex
	windows map[string]*[60]tpsBucket
	now     func() int64
}

// NewTPSCounter constructs the counter.
func NewTPSCounter() *TPSCounter {
	return &TPSCounter{
		windows: make(map[string]*[60]tpsBucket),
		now:     func() int64 { return time.Now().Unix() },
	}
}

// Record increments the current-second bucket for a contestant.
func (t *TPSCounter) Record(contestantID string) {
	sec := t.now()
	idx := sec % 60
	t.mu.Lock()
	defer t.mu.Unlock()
	w := t.windows[contestantID]
	if w == nil {
		w = new([60]tpsBucket)
		t.windows[contestantID] = w
	}
	b := &w[idx]
	if b.sec != sec {
		b.sec = sec
		b.count = 0
	}
	b.count++
}

// GetCurrentTPS returns the smoothed TPS over the five seconds before now.
// Buckets whose tagged second falls outside that range count as zero.
func (t *TPSCounter) GetCurrentTPS(contestantID string) float64 {
	sec := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	w := t.windows[contestantID]
	if w == nil {
		return 0
	}
	var sum int64
	for i := int64(1); i <= 5; i++ {
		want := sec - i
		b := w[((want)%60+60)%60]
		if b.sec == want {
			sum += b.count
		}
	}
	return float64(sum) / 5.0
}

// GetPeakTPS returns the highest single-second count within the trailing 60s
// window. Buckets older than the window are ignored.
func (t *TPSCounter) GetPeakTPS(contestantID string) float64 {
	sec := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	w := t.windows[contestantID]
	if w == nil {
		return 0
	}
	var max int64
	for _, b := range w {
		if b.sec > sec-60 && b.sec <= sec && b.count > max {
			max = b.count
		}
	}
	return float64(max)
}
