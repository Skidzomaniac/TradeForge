package latency

import "sync"

const windowBuckets = 30 // 30 × 1s buckets

// SlidingWindowHistogram keeps a rolling window of per-second HDR histograms so
// percentiles reflect only the recent past.
type SlidingWindowHistogram struct {
	mu         sync.Mutex
	buckets    [windowBuckets]*HDRHistogram
	currentIdx int
}

// NewSlidingWindow constructs the rolling histogram.
func NewSlidingWindow() *SlidingWindowHistogram {
	s := &SlidingWindowHistogram{}
	for i := range s.buckets {
		s.buckets[i] = NewHDRHistogram()
	}
	return s
}

// Record adds a sample to the current bucket.
func (s *SlidingWindowHistogram) Record(latencyUs int64) {
	s.mu.Lock()
	s.buckets[s.currentIdx].RecordValue(latencyUs)
	s.mu.Unlock()
}

// Advance rotates to the next bucket (call once per second).
func (s *SlidingWindowHistogram) Advance() {
	s.mu.Lock()
	s.currentIdx = (s.currentIdx + 1) % windowBuckets
	s.buckets[s.currentIdx].Reset()
	s.mu.Unlock()
}

// GetPercentiles merges all buckets except the in-progress one and returns
// p50/p90/p99.
func (s *SlidingWindowHistogram) GetPercentiles() (p50, p90, p99 int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	merged := NewHDRHistogram()
	for i, b := range s.buckets {
		if i == s.currentIdx {
			continue
		}
		merged.MergeFrom(b)
	}
	// Include the current bucket too, so very fresh data still counts.
	merged.MergeFrom(s.buckets[s.currentIdx])
	return merged.Percentile(0.50), merged.Percentile(0.90), merged.Percentile(0.99)
}
