package shadowbook

import (
	"testing"

	"github.com/trade-eval/telemetry-ingester/model"
)

// corrEvent builds a serialized correctness-phase event.
func corrEvent(seq int64, id, typ string, price, qty float64, actual model.Fill) model.TelemetryEvent {
	return model.TelemetryEvent{
		ContestantID: "c", TestID: "t1", Phase: model.PhaseCorrectness,
		OrderID: id, OrderType: typ, Price: price, Quantity: qty,
		SequenceNumber: seq, ActualFill: actual,
	}
}

// serializedScript returns the (orders, correctActuals) for a deterministic
// in-order stream that exercises every reachable outcome. The reference book
// computes the expected outcome; correctActuals mirror it exactly.
func serializedScript() []model.TelemetryEvent {
	return []model.TelemetryEvent{
		corrEvent(1, "a1", "LIMIT_SELL", 101, 10, model.Fill{Status: "PENDING"}),
		corrEvent(2, "a2", "LIMIT_SELL", 102, 10, model.Fill{Status: "PENDING"}),
		corrEvent(3, "b1", "LIMIT_BUY", 99, 10, model.Fill{Status: "PENDING"}),
		corrEvent(4, "x1", "LIMIT_BUY", 101, 5, model.Fill{Status: "FILLED", Price: 101, Quantity: 5}),
		corrEvent(5, "m1", "MARKET_BUY", 0, 5, model.Fill{Status: "FILLED", Price: 101, Quantity: 5}),
		corrEvent(6, "m2", "MARKET_BUY", 0, 15, model.Fill{Status: "PARTIAL", Price: 102, Quantity: 10}),
		corrEvent(7, "m3", "MARKET_BUY", 0, 5, model.Fill{Status: "REJECTED"}),
		corrEvent(8, "c1", "CANCEL", 0, 0, model.Fill{Status: "CANCELLED"}),
		corrEvent(9, "c2", "CANCEL", 0, 0, model.Fill{Status: "NOT_FOUND"}),
	}
}

func TestSerializedCorrectRunScoresOne(t *testing.T) {
	v := NewCorrectnessValidator()
	evs := serializedScript()
	// Point both cancels at the resting bid b1.
	evs[7].OrderID = "b1"
	evs[8].OrderID = "b1"
	res := v.ValidateBatch(evs)
	for i, r := range res {
		if !r.Correct {
			t.Fatalf("event %d (%s) should be correct: %s", i, evs[i].OrderType, r.Reason)
		}
		if r.HardViolation {
			t.Fatalf("event %d should not be a hard violation: %s", i, r.Reason)
		}
	}
}

func TestAlwaysFilledCheaterScoresNearZero(t *testing.T) {
	v := NewCorrectnessValidator()
	evs := serializedScript()
	evs[7].OrderID = "b1"
	evs[8].OrderID = "b1"
	// The cheater returns FILLED for every order regardless of truth.
	for i := range evs {
		evs[i].ActualFill = model.Fill{Status: "FILLED", Price: evs[i].Price, Quantity: evs[i].Quantity}
	}
	res := v.ValidateBatch(evs)
	correct := 0
	for _, r := range res {
		if r.Correct {
			correct++
		}
	}
	// At most the genuinely-filled orders (x1, m1) could coincidentally match;
	// everything else must be wrong. Far from a passing score.
	if correct > 2 {
		t.Fatalf("always-FILLED cheater scored %d/%d correct, want <= 2", correct, len(res))
	}
}

func TestHardViolation_Overfill(t *testing.T) {
	v := NewCorrectnessValidator()
	evs := []model.TelemetryEvent{
		corrEvent(1, "a1", "LIMIT_SELL", 100, 5, model.Fill{Status: "PENDING"}),
		// Buy 5 but claim a fill of 50: cannot fill more than submitted.
		corrEvent(2, "b1", "LIMIT_BUY", 100, 5, model.Fill{Status: "FILLED", Price: 100, Quantity: 50}),
	}
	res := v.ValidateBatch(evs)
	if !res[1].HardViolation {
		t.Fatalf("expected overfill hard violation, got %+v", res[1])
	}
}

func TestHardViolation_ImpossiblePrice(t *testing.T) {
	v := NewCorrectnessValidator()
	evs := []model.TelemetryEvent{
		corrEvent(1, "a1", "LIMIT_SELL", 100, 5, model.Fill{Status: "PENDING"}),
		// Fill reported at 250, a price never quoted.
		corrEvent(2, "b1", "LIMIT_BUY", 100, 5, model.Fill{Status: "FILLED", Price: 250, Quantity: 5}),
	}
	res := v.ValidateBatch(evs)
	if !res[1].HardViolation {
		t.Fatalf("expected impossible-price hard violation, got %+v", res[1])
	}
}

func TestHardViolation_ImpossibleCross(t *testing.T) {
	v := NewCorrectnessValidator()
	evs := []model.TelemetryEvent{
		// Only a high ask exists; a buy below it cannot cross.
		corrEvent(1, "a1", "LIMIT_SELL", 200, 5, model.Fill{Status: "PENDING"}),
		corrEvent(2, "b1", "LIMIT_BUY", 100, 5, model.Fill{Status: "FILLED", Price: 100, Quantity: 5}),
	}
	res := v.ValidateBatch(evs)
	if !res[1].HardViolation {
		t.Fatalf("expected impossible-cross hard violation, got %+v", res[1])
	}
}

func TestHardViolation_FillWithNoLiquidity(t *testing.T) {
	v := NewCorrectnessValidator()
	evs := []model.TelemetryEvent{
		// No asks ever existed, yet a market buy claims a fill.
		corrEvent(1, "m1", "MARKET_BUY", 0, 5, model.Fill{Status: "FILLED", Price: 100, Quantity: 5}),
	}
	res := v.ValidateBatch(evs)
	if !res[0].HardViolation {
		t.Fatalf("expected no-liquidity hard violation, got %+v", res[0])
	}
}

func TestHardViolation_NegativeQuantity(t *testing.T) {
	v := NewCorrectnessValidator()
	evs := []model.TelemetryEvent{
		corrEvent(1, "m1", "MARKET_BUY", 0, 5, model.Fill{Status: "FILLED", Price: 100, Quantity: -5}),
	}
	res := v.ValidateBatch(evs)
	if !res[0].HardViolation {
		t.Fatalf("expected negative-quantity hard violation, got %+v", res[0])
	}
}

func TestLoadPhaseToleratesOrderingButNotHardViolations(t *testing.T) {
	v := NewCorrectnessValidator()
	// Load-phase events: a status disagreement that is merely ordering-sensitive
	// is tolerated; an impossible fill is not.
	evs := []model.TelemetryEvent{
		{ContestantID: "c", TestID: "t", Phase: model.PhaseLoad, OrderID: "a1", OrderType: "LIMIT_SELL", Price: 100, Quantity: 5, SequenceNumber: 1, ActualFill: model.Fill{Status: "PENDING"}},
		// Tolerated: reference would rest this buy (PENDING) but the server filled
		// it against liquidity that legitimately existed (ask@100). Not a hard
		// violation.
		{ContestantID: "c", TestID: "t", Phase: model.PhaseLoad, OrderID: "b1", OrderType: "LIMIT_BUY", Price: 100, Quantity: 5, SequenceNumber: 2, ActualFill: model.Fill{Status: "FILLED", Price: 100, Quantity: 5}},
		// Not tolerated: fill at a price never quoted.
		{ContestantID: "c", TestID: "t", Phase: model.PhaseLoad, OrderID: "b2", OrderType: "LIMIT_BUY", Price: 100, Quantity: 5, SequenceNumber: 3, ActualFill: model.Fill{Status: "FILLED", Price: 999, Quantity: 5}},
	}
	res := v.ValidateBatch(evs)
	if !res[1].Correct || res[1].HardViolation {
		t.Fatalf("ordering-sensitive load event should be tolerated: %+v", res[1])
	}
	if !res[2].HardViolation {
		t.Fatalf("impossible-price load event should be a hard violation: %+v", res[2])
	}
}

func TestCancelSemantics(t *testing.T) {
	b := NewOrderBook()
	b.ProcessOrder(Order{ID: "r1", Type: "LIMIT_BUY", Price: 99, Quantity: 5})
	if f := b.ProcessOrder(Order{ID: "r1", Type: "CANCEL"}); f.Status != StatusCancelled {
		t.Fatalf("resting cancel: got %s want CANCELLED", f.Status)
	}
	if f := b.ProcessOrder(Order{ID: "r1", Type: "CANCEL"}); f.Status != StatusNotFound {
		t.Fatalf("re-cancel of removed order: got %s want NOT_FOUND", f.Status)
	}
	// A fully filled order, then a cancel: ALREADY_FILLED.
	b.ProcessOrder(Order{ID: "s1", Type: "LIMIT_SELL", Price: 100, Quantity: 5})
	b.ProcessOrder(Order{ID: "x1", Type: "LIMIT_BUY", Price: 100, Quantity: 5}) // fills x1 fully
	if f := b.ProcessOrder(Order{ID: "x1", Type: "CANCEL"}); f.Status != StatusAlreadyFilled {
		t.Fatalf("cancel of filled order: got %s want ALREADY_FILLED", f.Status)
	}
}
