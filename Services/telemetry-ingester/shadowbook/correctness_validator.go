package shadowbook

import (
	"sort"

	"github.com/trade-eval/telemetry-ingester/model"
)

const (
	qtyEpsilon   = 1e-6
	priceEpsilon = 0.01 // one cent
)

// CorrectnessValidator replays events through the authoritative book in
// sequence order and judges correctness in two distinct ways:
//
//   - In the serialized correctness phase the platform drives the contestant
//     with a single in-order stream, so the true processing order is known and
//     the comparison against the reference is exact: status must match, filled
//     price must be within one cent, filled quantity must match.
//   - In the concurrent load phase the true processing order is not observable,
//     so correctness is judged only by ordering-independent hard-violation
//     checks. Legitimate ordering-sensitive disagreements are tolerated.
//
// Hard-violation checks run in every phase and always count against a
// contestant: an overfill (filling more than was submitted), a fill at a price
// never quoted, a fill on a side that could never have crossed, or a negative
// quantity. These are impossible for any honest implementation regardless of
// internal ordering, which is what closes the "return FILLED for everything"
// loophole.
type CorrectnessValidator struct {
	books      map[string]*OrderBook // per contestant
	lastTestID map[string]string     // contestant -> last seen test_id
}

// NewCorrectnessValidator constructs the validator.
func NewCorrectnessValidator() *CorrectnessValidator {
	return &CorrectnessValidator{
		books:      make(map[string]*OrderBook),
		lastTestID: make(map[string]string),
	}
}

// ValidateBatch processes events (sorted by sequence number) and returns a
// verdict per input event.
func (v *CorrectnessValidator) ValidateBatch(events []model.TelemetryEvent) []model.ValidationResult {
	sorted := make([]model.TelemetryEvent, len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].SequenceNumber < sorted[j].SequenceNumber
	})

	results := make([]model.ValidationResult, len(sorted))
	for i, e := range sorted {
		// Create the book on first sight of a contestant, and reset it when a
		// new test starts. Resetting keeps the shadow book in sync with the
		// contestant book, which the bot-fleet clears via POST /reset per test.
		book := v.books[e.ContestantID]
		prev, seen := v.lastTestID[e.ContestantID]
		if book == nil || (seen && prev != e.TestID) {
			book = NewOrderBook()
			v.books[e.ContestantID] = book
		}
		v.lastTestID[e.ContestantID] = e.TestID

		expected := book.ProcessOrder(Order{
			ID:           e.OrderID,
			ContestantID: e.ContestantID,
			Type:         e.OrderType,
			Price:        e.Price,
			Quantity:     e.Quantity,
			SubmittedAt:  e.SequenceNumber,
		})
		results[i] = validate(book, e, expected)
	}
	return results
}

// validate applies the hard-violation checks (all phases) and, in the
// correctness phase, the exact comparison.
func validate(book *OrderBook, e model.TelemetryEvent, expected ExpectedFill) model.ValidationResult {
	if reason := hardViolation(book, e); reason != "" {
		return model.ValidationResult{Correct: false, HardViolation: true, Reason: reason}
	}
	if e.Phase == model.PhaseCorrectness {
		return exactCompare(expected, e.ActualFill)
	}
	// Load phase, no hard violation: ordering-sensitive disagreements are
	// tolerated and do not count against correctness.
	return model.ValidationResult{Correct: true}
}

// hardViolation returns a non-empty reason if the contestant's reported fill is
// impossible regardless of how a concurrent server interleaved its processing.
// It uses facts accumulated over the whole test (the lowest ask and highest bid
// ever quoted, every price ever quoted), so it does not depend on the exact
// interleaving the validator happened to replay. The order has already been
// processed through the book, so its own quote is reflected in the facts.
func hardViolation(book *OrderBook, e model.TelemetryEvent) string {
	a := e.ActualFill
	if a.Quantity < -qtyEpsilon {
		return "negative quantity"
	}
	isCancel := e.OrderType == "CANCEL"
	// You can never fill more than you submitted. Ordering-independent.
	if !isCancel && e.Quantity > 0 && a.Quantity > e.Quantity+qtyEpsilon {
		return "overfill: filled more than submitted"
	}
	// The remaining checks apply only to a claimed fill.
	if a.Status != StatusFilled && a.Status != StatusPartial {
		return ""
	}
	facts := book.Facts()
	switch e.OrderType {
	case "MARKET_BUY", "LIMIT_BUY":
		if !facts.AnyAsk {
			return "impossible fill: no ask liquidity ever existed"
		}
		if e.OrderType == "LIMIT_BUY" && e.Price > 0 && facts.MinAskEver > e.Price+priceEpsilon {
			return "impossible cross: buy price below the lowest ask ever quoted"
		}
	case "MARKET_SELL", "LIMIT_SELL":
		if !facts.AnyBid {
			return "impossible fill: no bid liquidity ever existed"
		}
		if e.OrderType == "LIMIT_SELL" && e.Price > 0 && facts.MaxBidEver < e.Price-priceEpsilon {
			return "impossible cross: sell price above the highest bid ever quoted"
		}
	}
	// A fill can only occur at a resting limit price. A fill at a price that was
	// never quoted as a limit order is fabricated.
	if a.Price > 0 && !book.PriceEverQuoted(a.Price) {
		return "impossible price: fill at a price never quoted in the book"
	}
	return ""
}

// exactCompare is the strict comparison used in the serialized correctness
// phase. Status must match exactly; for fills, quantity must match and price
// must be within one cent. There are no status tolerations here: that is what
// makes an always-FILLED submission score near zero.
func exactCompare(expected ExpectedFill, actual model.Fill) model.ValidationResult {
	if actual.Status == "" {
		return model.ValidationResult{Correct: false, Reason: "missing actual fill"}
	}
	if expected.Status != actual.Status {
		return model.ValidationResult{Correct: false, Reason: "status mismatch: expected " + expected.Status + " got " + actual.Status}
	}
	if expected.Status == StatusFilled || expected.Status == StatusPartial {
		if d := expected.FilledQty - actual.Quantity; d > qtyEpsilon || d < -qtyEpsilon {
			return model.ValidationResult{Correct: false, Reason: "filled quantity mismatch"}
		}
		// A fill must report its real execution price. Reporting 0 (or a price
		// off by more than a cent) is a mismatch.
		if expected.FilledPrice > 0 {
			if d := expected.FilledPrice - actual.Price; d > priceEpsilon || d < -priceEpsilon {
				return model.ValidationResult{Correct: false, Reason: "filled price mismatch"}
			}
		}
	}
	return model.ValidationResult{Correct: true}
}
