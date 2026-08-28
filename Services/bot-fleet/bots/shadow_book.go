package bots

import (
	"sort"
	"sync"
	"time"
)

// ExpectedFill is what a correct order book should return for an order.
type ExpectedFill struct {
	Status   string // FILLED/PARTIAL/PENDING/CANCELLED/REJECTED
	Price    float64
	Quantity float64
	Remaining float64
}

// Order is a resting order in the shadow book.
type Order struct {
	OrderID  string
	Type     string
	Price    float64
	Quantity float64
	Time     time.Time
}

type priceLevel struct {
	Price  float64
	Orders []Order
}

// ShadowBook is a lightweight single-bot local order-book tracker used to form
// fill expectations. It is NOT the authoritative validator.
type ShadowBook struct {
	mu   sync.Mutex
	bids []priceLevel // descending price
	asks []priceLevel // ascending price
}

// NewShadowBook constructs an empty book.
func NewShadowBook() *ShadowBook { return &ShadowBook{} }

// AddOrder returns the expected fill for submitting order, mutating the book.
func (b *ShadowBook) AddOrder(o Order) ExpectedFill {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch o.Type {
	case "MARKET_BUY":
		return b.matchMarket(&b.asks, o.Quantity)
	case "MARKET_SELL":
		return b.matchMarket(&b.bids, o.Quantity)
	case "LIMIT_BUY":
		f := b.matchLimit(&b.asks, o, true)
		if f.Remaining > 0 {
			b.insert(&b.bids, Order{o.OrderID, o.Type, o.Price, f.Remaining, o.Time}, false)
		}
		return f
	case "LIMIT_SELL":
		f := b.matchLimit(&b.bids, o, false)
		if f.Remaining > 0 {
			b.insert(&b.asks, Order{o.OrderID, o.Type, o.Price, f.Remaining, o.Time}, true)
		}
		return f
	case "CANCEL":
		b.cancel(o.OrderID)
		return ExpectedFill{Status: "CANCELLED"}
	}
	return ExpectedFill{Status: "REJECTED"}
}

func (b *ShadowBook) matchMarket(levels *[]priceLevel, qty float64) ExpectedFill {
	filled, lastPx := 0.0, 0.0
	for len(*levels) > 0 && qty > 0 {
		lvl := &(*levels)[0]
		for len(lvl.Orders) > 0 && qty > 0 {
			m := min(qty, lvl.Orders[0].Quantity)
			filled += m
			lastPx = lvl.Price
			qty -= m
			lvl.Orders[0].Quantity -= m
			if lvl.Orders[0].Quantity == 0 {
				lvl.Orders = lvl.Orders[1:]
			}
		}
		if len(lvl.Orders) == 0 {
			*levels = (*levels)[1:]
		}
	}
	if filled == 0 {
		return ExpectedFill{Status: "REJECTED"}
	}
	status := "FILLED"
	if qty > 0 {
		status = "PARTIAL"
	}
	return ExpectedFill{Status: status, Price: lastPx, Quantity: filled, Remaining: qty}
}

func (b *ShadowBook) matchLimit(levels *[]priceLevel, o Order, isBuy bool) ExpectedFill {
	filled, lastPx := 0.0, 0.0
	remaining := o.Quantity
	for len(*levels) > 0 && remaining > 0 {
		lvl := &(*levels)[0]
		crosses := (isBuy && lvl.Price <= o.Price) || (!isBuy && lvl.Price >= o.Price)
		if !crosses {
			break
		}
		for len(lvl.Orders) > 0 && remaining > 0 {
			m := min(remaining, lvl.Orders[0].Quantity)
			filled += m
			lastPx = lvl.Price
			remaining -= m
			lvl.Orders[0].Quantity -= m
			if lvl.Orders[0].Quantity == 0 {
				lvl.Orders = lvl.Orders[1:]
			}
		}
		if len(lvl.Orders) == 0 {
			*levels = (*levels)[1:]
		}
	}
	if filled == 0 {
		return ExpectedFill{Status: "PENDING", Remaining: remaining}
	}
	status := "FILLED"
	if remaining > 0 {
		status = "PARTIAL"
	}
	return ExpectedFill{Status: status, Price: lastPx, Quantity: filled, Remaining: remaining}
}

func (b *ShadowBook) insert(levels *[]priceLevel, o Order, ascending bool) {
	for i := range *levels {
		if (*levels)[i].Price == o.Price {
			(*levels)[i].Orders = append((*levels)[i].Orders, o)
			return
		}
	}
	*levels = append(*levels, priceLevel{Price: o.Price, Orders: []Order{o}})
	sort.Slice(*levels, func(i, j int) bool {
		if ascending {
			return (*levels)[i].Price < (*levels)[j].Price
		}
		return (*levels)[i].Price > (*levels)[j].Price
	})
}

func (b *ShadowBook) cancel(orderID string) bool {
	for _, set := range []*[]priceLevel{&b.bids, &b.asks} {
		for i := range *set {
			for j, o := range (*set)[i].Orders {
				if o.OrderID == orderID {
					(*set)[i].Orders = append((*set)[i].Orders[:j], (*set)[i].Orders[j+1:]...)
					return true
				}
			}
		}
	}
	return false
}

// ValidateFill compares an expected fill to the contestant's actual fill,
// catching obvious violations (wrong status, wrong limit price, overfill).
func ValidateFill(expected ExpectedFill, actualStatus string, actualPrice, actualQty float64) (bool, string) {
	if expected.Status != actualStatus {
		// Allow PARTIAL where FILLED expected only if quantities are sane.
		if !(expected.Status == "FILLED" && actualStatus == "PARTIAL") {
			return false, "status mismatch"
		}
	}
	if actualQty > expected.Quantity+1e-9 && expected.Status != "PENDING" {
		return false, "overfill"
	}
	if actualQty < 0 {
		return false, "negative quantity"
	}
	if expected.Status == "FILLED" && expected.Price > 0 && actualPrice > 0 {
		// limit fills must match price exactly
		if diff := expected.Price - actualPrice; diff > 0.01 || diff < -0.01 {
			return false, "wrong price"
		}
	}
	return true, ""
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
