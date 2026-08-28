package latency

import (
	"math"
	"testing"
)

// TestMemoryBounded asserts a single histogram stays in the tens of kilobytes
// regardless of how many values are recorded. The old flat array was about
// 80 MB each.
func TestMemoryBounded(t *testing.T) {
	h := NewHDRHistogram()
	for i := int64(0); i < 1_000_000; i++ {
		h.RecordValue(i % highestTrackableValue)
	}
	bytes := len(h.counts) * 8
	if bytes > 64*1024 {
		t.Fatalf("counts array is %d bytes, want <= 64 KB", bytes)
	}
	if bytes == 0 {
		t.Fatal("counts array is empty")
	}
	t.Logf("per-histogram counts memory: %d bytes (%d buckets)", bytes, len(h.counts))
}

// TestPercentileAccuracy records a known ramp distribution and asserts the
// reported percentiles land within the histogram's relative precision bound.
func TestPercentileAccuracy(t *testing.T) {
	h := NewHDRHistogram()
	const n = 100_000
	for v := int64(1); v <= n; v++ {
		h.RecordValue(v)
	}

	cases := []struct {
		pct  float64
		want float64
	}{
		{0.50, 50_000},
		{0.90, 90_000},
		{0.99, 99_000},
		{0.999, 99_900},
	}
	// Two significant digits guarantees better than 2% relative error; allow a
	// small margin for the discrete sampling of the ramp.
	const tolerance = 0.02
	for _, c := range cases {
		got := float64(h.Percentile(c.pct))
		relErr := math.Abs(got-c.want) / c.want
		if relErr > tolerance {
			t.Errorf("p%.1f = %.0f, want ~%.0f (rel err %.4f > %.2f)",
				c.pct*100, got, c.want, relErr, tolerance)
		}
	}
}

// TestValueRoundTrip asserts that recording a value and reading it back at the
// p100 lands within the precision bound for values across the whole range.
func TestValueRoundTrip(t *testing.T) {
	for _, v := range []int64{1, 50, 100, 999, 1000, 12_345, 250_000, 9_999_999} {
		h := NewHDRHistogram()
		h.RecordValue(v)
		got := h.Percentile(1.0)
		relErr := math.Abs(float64(got-v)) / float64(v)
		if relErr > 0.02 {
			t.Errorf("recorded %d, read back %d (rel err %.4f)", v, got, relErr)
		}
	}
}

// TestClampAboveMax asserts values above the tracked range are clamped, not
// dropped or panicking.
func TestClampAboveMax(t *testing.T) {
	h := NewHDRHistogram()
	h.RecordValue(highestTrackableValue * 100)
	if h.Count() != 1 {
		t.Fatalf("count = %d, want 1", h.Count())
	}
	got := h.Percentile(0.50)
	relErr := math.Abs(float64(got-highestTrackableValue)) / float64(highestTrackableValue)
	if relErr > 0.02 {
		t.Errorf("clamped value read back as %d, want ~%d", got, highestTrackableValue)
	}
}

// TestMergeFrom asserts merged percentiles match recording into one histogram.
func TestMergeFrom(t *testing.T) {
	a := NewHDRHistogram()
	b := NewHDRHistogram()
	combined := NewHDRHistogram()
	for v := int64(1); v <= 1000; v++ {
		a.RecordValue(v)
		combined.RecordValue(v)
	}
	for v := int64(1001); v <= 2000; v++ {
		b.RecordValue(v)
		combined.RecordValue(v)
	}
	a.MergeFrom(b)
	if a.Count() != combined.Count() {
		t.Fatalf("merged count %d, want %d", a.Count(), combined.Count())
	}
	for _, pct := range []float64{0.5, 0.9, 0.99} {
		if a.Percentile(pct) != combined.Percentile(pct) {
			t.Errorf("p%.0f: merged=%d combined=%d", pct*100, a.Percentile(pct), combined.Percentile(pct))
		}
	}
}
