package metrics

import "testing"

func TestTPSCounterCountsRecentActivity(t *testing.T) {
	now := int64(1000)
	c := NewTPSCounter()
	c.now = func() int64 { return now }

	for i := 0; i < 10; i++ {
		c.Record("a")
	}
	now = 1001 // read the second after the activity
	if got := c.GetCurrentTPS("a"); got != 2.0 { // 10 over a 5s window
		t.Fatalf("want 2.0 tps, got %v", got)
	}
	if got := c.GetPeakTPS("a"); got != 10 {
		t.Fatalf("want peak 10, got %v", got)
	}
}

// TestTPSCounterStaleAfterIdle is the regression for item 12: after an idle gap
// longer than the window, both reads must be zero, not the leftover counts from
// an earlier minute.
func TestTPSCounterStaleAfterIdle(t *testing.T) {
	now := int64(1000)
	c := NewTPSCounter()
	c.now = func() int64 { return now }

	for i := 0; i < 50; i++ {
		c.Record("a")
	}
	now = 1000 + 200 // idle well past the 60s ring
	if got := c.GetCurrentTPS("a"); got != 0 {
		t.Fatalf("current TPS should be 0 after idle, got %v", got)
	}
	if got := c.GetPeakTPS("a"); got != 0 {
		t.Fatalf("peak TPS should be 0 after idle, got %v", got)
	}
}

// TestTPSCounterWrapAroundNotCountedAsCurrent guards the ring index collision:
// activity exactly 60s ago lands on the same bucket index but a different
// second, so it must not be read as current.
func TestTPSCounterWrapAroundNotCountedAsCurrent(t *testing.T) {
	now := int64(1000)
	c := NewTPSCounter()
	c.now = func() int64 { return now }
	c.Record("a") // bucket index 1000%60, tagged second 1000

	now = 1061 // 1061-1 = 1060 maps to same index as 1000 but is a later second
	if got := c.GetCurrentTPS("a"); got != 0 {
		t.Fatalf("stale wrap-around bucket counted as current: %v", got)
	}
}
