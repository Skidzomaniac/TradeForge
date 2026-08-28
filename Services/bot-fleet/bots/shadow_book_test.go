package bots

import (
	"testing"
	"time"
)

func TestLimitBuyFilledByAsk(t *testing.T) {
	b := NewShadowBook()
	b.AddOrder(Order{OrderID: "s1", Type: "LIMIT_SELL", Price: 100, Quantity: 10, Time: time.Now()})
	f := b.AddOrder(Order{OrderID: "b1", Type: "LIMIT_BUY", Price: 100, Quantity: 10, Time: time.Now()})
	if f.Status != "FILLED" || f.Price != 100 || f.Quantity != 10 {
		t.Fatalf("expected full fill at 100, got %+v", f)
	}
}

func TestLimitBuyRests(t *testing.T) {
	b := NewShadowBook()
	f := b.AddOrder(Order{OrderID: "b1", Type: "LIMIT_BUY", Price: 99, Quantity: 5, Time: time.Now()})
	if f.Status != "PENDING" {
		t.Fatalf("expected PENDING, got %+v", f)
	}
}

func TestMarketBuyMultipleLevels(t *testing.T) {
	b := NewShadowBook()
	b.AddOrder(Order{OrderID: "s1", Type: "LIMIT_SELL", Price: 100, Quantity: 10, Time: time.Now()})
	b.AddOrder(Order{OrderID: "s2", Type: "LIMIT_SELL", Price: 101, Quantity: 10, Time: time.Now()})
	f := b.AddOrder(Order{OrderID: "m1", Type: "MARKET_BUY", Quantity: 15, Time: time.Now()})
	if f.Status != "FILLED" || f.Quantity != 15 {
		t.Fatalf("expected 15 filled, got %+v", f)
	}
}

func TestCancelRemoves(t *testing.T) {
	b := NewShadowBook()
	b.AddOrder(Order{OrderID: "b1", Type: "LIMIT_BUY", Price: 50, Quantity: 5, Time: time.Now()})
	f := b.AddOrder(Order{OrderID: "b1", Type: "CANCEL"})
	if f.Status != "CANCELLED" {
		t.Fatalf("expected CANCELLED, got %+v", f)
	}
}

func TestValidateFill_Overfill(t *testing.T) {
	ok, reason := ValidateFill(ExpectedFill{Status: "FILLED", Quantity: 10, Price: 100}, "FILLED", 100, 20)
	if ok {
		t.Fatalf("expected overfill rejection, got ok (%s)", reason)
	}
}
