package latency

import "testing"

func TestPercentile_KnownValues(t *testing.T) {
	h := NewHDRHistogram()
	for i := 0; i < 500; i++ {
		h.RecordValue(100)
	}
	for i := 0; i < 400; i++ {
		h.RecordValue(500)
	}
	for i := 0; i < 99; i++ {
		h.RecordValue(1000)
	}
	h.RecordValue(10000)

	if p := h.Percentile(0.50); p != 100 {
		t.Errorf("p50 = %d, want 100", p)
	}
	if p := h.Percentile(0.99); p != 1000 {
		t.Errorf("p99 = %d, want 1000", p)
	}
}

func TestSlidingWindow_OldDataExpires(t *testing.T) {
	s := NewSlidingWindow()
	for i := 0; i < 1000; i++ {
		s.Record(100)
	}
	for i := 0; i < windowBuckets+1; i++ {
		s.Advance()
	}
	p50, _, _ := s.GetPercentiles()
	if p50 != 0 {
		t.Errorf("expected expired data (p50=0), got %d", p50)
	}
}

func TestReset(t *testing.T) {
	h := NewHDRHistogram()
	h.RecordValue(50)
	h.Reset()
	if h.Count() != 0 {
		t.Errorf("expected 0 after reset, got %d", h.Count())
	}
}
