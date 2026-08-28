package metrics

import (
	"testing"

	"github.com/trade-eval/telemetry-ingester/model"
)

func TestCorrectnessRate(t *testing.T) {
	r := NewRegistry()
	a := r.Get("c1")
	// Correctness is scored from the serialized correctness phase.
	for i := 0; i < 500; i++ {
		a.ProcessEvent(model.TelemetryEvent{ContestantID: "c1", LatencyUs: 100, Phase: model.PhaseCorrectness}, model.ValidationResult{Correct: true})
	}
	for i := 0; i < 500; i++ {
		a.ProcessEvent(model.TelemetryEvent{ContestantID: "c1", LatencyUs: 100, Phase: model.PhaseCorrectness}, model.ValidationResult{Correct: false})
	}
	s := a.GetSnapshot()
	if s.CorrectnessRate < 0.49 || s.CorrectnessRate > 0.51 {
		t.Fatalf("expected ~0.5, got %v", s.CorrectnessRate)
	}
}

// TestLoadPhaseDoesNotInflateCorrectness asserts that ordering-sensitive
// load-phase events (no hard violation) do not move the correctness rate, while
// a hard violation in the load phase does count against it.
func TestLoadPhaseDoesNotInflateCorrectness(t *testing.T) {
	r := NewRegistry()
	a := r.Get("c1")
	// One correct serialized event: rate should be 1.0.
	a.ProcessEvent(model.TelemetryEvent{ContestantID: "c1", Phase: model.PhaseCorrectness}, model.ValidationResult{Correct: true})
	// A flood of tolerated load events must not change the rate.
	for i := 0; i < 1000; i++ {
		a.ProcessEvent(model.TelemetryEvent{ContestantID: "c1", Phase: model.PhaseLoad}, model.ValidationResult{Correct: true})
	}
	if s := a.GetSnapshot(); s.CorrectnessRate != 1.0 {
		t.Fatalf("load events changed the rate: got %v, want 1.0", s.CorrectnessRate)
	}
	// A load-phase hard violation must count against correctness.
	a.ProcessEvent(model.TelemetryEvent{ContestantID: "c1", Phase: model.PhaseLoad}, model.ValidationResult{HardViolation: true})
	s := a.GetSnapshot()
	if s.HardViolations != 1 {
		t.Fatalf("hard violation not counted: %d", s.HardViolations)
	}
	if s.CorrectnessRate != 0.5 { // 1 correct of 2 counted
		t.Fatalf("expected 0.5 after hard violation, got %v", s.CorrectnessRate)
	}
}

// TestTimedOutExcludedFromCorrectness asserts timed-out orders do not count.
func TestTimedOutExcludedFromCorrectness(t *testing.T) {
	r := NewRegistry()
	a := r.Get("c1")
	a.ProcessEvent(model.TelemetryEvent{ContestantID: "c1", Phase: model.PhaseCorrectness}, model.ValidationResult{Correct: true})
	for i := 0; i < 100; i++ {
		a.ProcessEvent(model.TelemetryEvent{ContestantID: "c1", Phase: model.PhaseCorrectness, TimedOut: true}, model.ValidationResult{})
	}
	s := a.GetSnapshot()
	if s.CorrectnessRate != 1.0 {
		t.Fatalf("timed-out orders affected the rate: %v", s.CorrectnessRate)
	}
	if s.ValidOrders != 1 {
		t.Fatalf("expected 1 valid order, got %d", s.ValidOrders)
	}
}

func TestTPSCounter_Counts(t *testing.T) {
	c := NewTPSCounter()
	for i := 0; i < 1000; i++ {
		c.Record("c1")
	}
	if peak := c.GetPeakTPS("c1"); peak < 1 {
		t.Fatalf("expected peak >= 1, got %v", peak)
	}
}
