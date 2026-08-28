package shadowbook

import (
	"testing"

	"github.com/trade-eval/telemetry-ingester/model"
)

func TestPriceTimePriority(t *testing.T) {
	b := NewOrderBook()
	b.ProcessOrder(Order{ID: "s1", Type: "LIMIT_SELL", Price: 100, Quantity: 5, SubmittedAt: 1})
	b.ProcessOrder(Order{ID: "s2", Type: "LIMIT_SELL", Price: 100, Quantity: 5, SubmittedAt: 2})
	// A buy of 5 should consume s1 first (earlier).
	f := b.ProcessOrder(Order{ID: "b1", Type: "LIMIT_BUY", Price: 100, Quantity: 5, SubmittedAt: 3})
	if f.Status != StatusFilled || f.FilledQty != 5 {
		t.Fatalf("unexpected fill %+v", f)
	}
	if _, ok := b.orders["s1"]; ok {
		t.Fatal("s1 should be consumed first")
	}
	if _, ok := b.orders["s2"]; !ok {
		t.Fatal("s2 should remain")
	}
}

func TestMarketBuyConsumesLevels(t *testing.T) {
	b := NewOrderBook()
	b.ProcessOrder(Order{ID: "s1", Type: "LIMIT_SELL", Price: 100, Quantity: 10, SubmittedAt: 1})
	b.ProcessOrder(Order{ID: "s2", Type: "LIMIT_SELL", Price: 101, Quantity: 10, SubmittedAt: 2})
	f := b.ProcessOrder(Order{ID: "m1", Type: "MARKET_BUY", Quantity: 15, SubmittedAt: 3})
	if f.Status != StatusFilled || f.FilledQty != 15 {
		t.Fatalf("unexpected %+v", f)
	}
}

func TestLimitBuyRests(t *testing.T) {
	b := NewOrderBook()
	f := b.ProcessOrder(Order{ID: "b1", Type: "LIMIT_BUY", Price: 99, Quantity: 5, SubmittedAt: 1})
	if f.Status != StatusPending {
		t.Fatalf("expected pending, got %+v", f)
	}
}

func TestValidator_WrongPrice(t *testing.T) {
	v := NewCorrectnessValidator()
	events := []model.TelemetryEvent{
		{ContestantID: "c", OrderID: "s1", OrderType: "LIMIT_SELL", Price: 100, Quantity: 10, SequenceNumber: 1,
			ActualFill: model.Fill{Status: "PENDING"}},
		{ContestantID: "c", OrderID: "b1", OrderType: "LIMIT_BUY", Price: 100, Quantity: 10, SequenceNumber: 2,
			ActualFill: model.Fill{Status: "FILLED", Price: 105, Quantity: 10}}, // wrong price
	}
	res := v.ValidateBatch(events)
	if res[1].Correct {
		t.Fatal("expected wrong-price detection")
	}
}

func TestValidator_CorrectFill(t *testing.T) {
	v := NewCorrectnessValidator()
	events := []model.TelemetryEvent{
		{ContestantID: "c", OrderID: "s1", OrderType: "LIMIT_SELL", Price: 100, Quantity: 10, SequenceNumber: 1,
			ActualFill: model.Fill{Status: "PENDING"}},
		{ContestantID: "c", OrderID: "b1", OrderType: "LIMIT_BUY", Price: 100, Quantity: 10, SequenceNumber: 2,
			ActualFill: model.Fill{Status: "FILLED", Price: 100, Quantity: 10}},
	}
	res := v.ValidateBatch(events)
	if !res[1].Correct {
		t.Fatalf("expected correct, got %s", res[1].Reason)
	}
}
