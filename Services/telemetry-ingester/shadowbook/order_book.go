// Package shadowbook is the authoritative price-time-priority matching engine
// used to validate contestant fills.
package shadowbook

import (
	"math"
	"sort"
	"sync"
)

// FillStatus values. These are the exact statuses the published API contract
// documents, so a contestant that follows the contract is never marked wrong.
const (
	StatusFilled        = "FILLED"
	StatusPartial       = "PARTIAL"
	StatusPending       = "PENDING"
	StatusRejected      = "REJECTED"
	StatusCancelled     = "CANCELLED"
	StatusNotFound      = "NOT_FOUND"
	StatusAlreadyFilled = "ALREADY_FILLED"
)

// priceCents converts a price to integer cents for use as a map key, avoiding
// float equality. The platform rounds all prices to two decimals.
func priceCents(price float64) int64 { return int64(math.Round(price * 100)) }

// Order is a tracked order.
type Order struct {
	ID           string
	ContestantID string
	Type         string
	Price        float64
	Quantity     float64
	RemainingQty float64
	SubmittedAt  int64 // sequence number, not wall clock
}

// ExpectedFill is the reference outcome for an order.
type ExpectedFill struct {
	Status       string
	FilledPrice  float64
	FilledQty    float64
	RemainingQty float64
}

type priceLevel struct {
	Price  float64
	Orders []*Order
}

// OrderBook is a price-time priority matching engine. Alongside matching it
// accumulates ordering-independent facts about the whole order stream
// (the liquidity that ever existed, every price ever quoted, every order that
// ever fully filled) so the validator can detect impossible fills regardless of
// how a concurrent server interleaved its processing.
type OrderBook struct {
	mu     sync.RWMutex
	bids   []priceLevel // descending
	asks   []priceLevel // ascending
	orders map[string]*Order

	// Hard-violation facts, accumulated across the whole test, never reset until
	// a new test starts (a fresh book is created).
	filled     map[string]bool  // order IDs that ever fully filled
	seenPrices map[int64]bool   // every limit price ever quoted, in cents
	minAskEver float64          // lowest LIMIT_SELL price ever quoted; +Inf if none
	maxBidEver float64          // highest LIMIT_BUY price ever quoted; -Inf if none
}

// NewOrderBook constructs an empty book.
func NewOrderBook() *OrderBook {
	return &OrderBook{
		orders:     make(map[string]*Order),
		filled:     make(map[string]bool),
		seenPrices: make(map[int64]bool),
		minAskEver: math.Inf(1),
		maxBidEver: math.Inf(-1),
	}
}

// ProcessOrder matches/rests an order and returns the expected fill.
func (b *OrderBook) ProcessOrder(o Order) ExpectedFill {
	b.mu.Lock()
	defer b.mu.Unlock()
	ord := &o
	ord.RemainingQty = o.Quantity

	// Record liquidity facts before matching. Every limit order is quoted
	// liquidity at its price regardless of whether it rests or crosses.
	switch o.Type {
	case "LIMIT_BUY":
		b.seenPrices[priceCents(o.Price)] = true
		if o.Price > b.maxBidEver {
			b.maxBidEver = o.Price
		}
	case "LIMIT_SELL":
		b.seenPrices[priceCents(o.Price)] = true
		if o.Price < b.minAskEver {
			b.minAskEver = o.Price
		}
	}

	var f ExpectedFill
	switch o.Type {
	case "MARKET_BUY":
		f = b.matchMarket(ord, &b.asks)
	case "MARKET_SELL":
		f = b.matchMarket(ord, &b.bids)
	case "LIMIT_BUY":
		f = b.matchLimit(ord, &b.asks, true)
		if ord.RemainingQty > 0 {
			b.insert(&b.bids, ord, false)
		}
	case "LIMIT_SELL":
		f = b.matchLimit(ord, &b.bids, false)
		if ord.RemainingQty > 0 {
			b.insert(&b.asks, ord, true)
		}
	case "CANCEL":
		return b.cancel(o.ID)
	default:
		return ExpectedFill{Status: StatusRejected}
	}

	// An incoming order that fully filled on arrival never rests, but a later
	// cancel of its ID must report ALREADY_FILLED, not NOT_FOUND.
	if f.Status == StatusFilled {
		b.filled[o.ID] = true
	}
	return f
}

func (b *OrderBook) matchMarket(o *Order, levels *[]priceLevel) ExpectedFill {
	filled, lastPx := 0.0, 0.0
	for len(*levels) > 0 && o.RemainingQty > 0 {
		lvl := &(*levels)[0]
		for len(lvl.Orders) > 0 && o.RemainingQty > 0 {
			resting := lvl.Orders[0]
			m := minF(o.RemainingQty, resting.RemainingQty)
			filled += m
			lastPx = lvl.Price
			o.RemainingQty -= m
			resting.RemainingQty -= m
			if resting.RemainingQty == 0 {
				delete(b.orders, resting.ID)
				b.filled[resting.ID] = true
				lvl.Orders = lvl.Orders[1:]
			}
		}
		if len(lvl.Orders) == 0 {
			*levels = (*levels)[1:]
		}
	}
	if filled == 0 {
		return ExpectedFill{Status: StatusRejected}
	}
	if o.RemainingQty > 0 {
		return ExpectedFill{Status: StatusPartial, FilledPrice: lastPx, FilledQty: filled, RemainingQty: o.RemainingQty}
	}
	return ExpectedFill{Status: StatusFilled, FilledPrice: lastPx, FilledQty: filled}
}

func (b *OrderBook) matchLimit(o *Order, levels *[]priceLevel, isBuy bool) ExpectedFill {
	filled, lastPx := 0.0, 0.0
	for len(*levels) > 0 && o.RemainingQty > 0 {
		lvl := &(*levels)[0]
		crosses := (isBuy && lvl.Price <= o.Price) || (!isBuy && lvl.Price >= o.Price)
		if !crosses {
			break
		}
		for len(lvl.Orders) > 0 && o.RemainingQty > 0 {
			resting := lvl.Orders[0]
			m := minF(o.RemainingQty, resting.RemainingQty)
			filled += m
			lastPx = lvl.Price
			o.RemainingQty -= m
			resting.RemainingQty -= m
			if resting.RemainingQty == 0 {
				delete(b.orders, resting.ID)
				b.filled[resting.ID] = true
				lvl.Orders = lvl.Orders[1:]
			}
		}
		if len(lvl.Orders) == 0 {
			*levels = (*levels)[1:]
		}
	}
	if filled == 0 {
		return ExpectedFill{Status: StatusPending, RemainingQty: o.RemainingQty}
	}
	if o.RemainingQty > 0 {
		// Multi-level sweep: the reported filled_price is the price of the last
		// level filled, by pinned contract convention.
		return ExpectedFill{Status: StatusPartial, FilledPrice: lastPx, FilledQty: filled, RemainingQty: o.RemainingQty}
	}
	return ExpectedFill{Status: StatusFilled, FilledPrice: lastPx, FilledQty: filled}
}

func (b *OrderBook) insert(levels *[]priceLevel, o *Order, ascending bool) {
	b.orders[o.ID] = o
	for i := range *levels {
		if (*levels)[i].Price == o.Price {
			(*levels)[i].Orders = append((*levels)[i].Orders, o) // FIFO by arrival
			return
		}
	}
	*levels = append(*levels, priceLevel{Price: o.Price, Orders: []*Order{o}})
	sort.SliceStable(*levels, func(i, j int) bool {
		if ascending {
			return (*levels)[i].Price < (*levels)[j].Price
		}
		return (*levels)[i].Price > (*levels)[j].Price
	})
}

// cancel removes a resting order. The returned status is pinned to the contract:
// CANCELLED if the order was resting, ALREADY_FILLED if it had fully filled, and
// NOT_FOUND if the book never saw it.
func (b *OrderBook) cancel(orderID string) ExpectedFill {
	if _, resting := b.orders[orderID]; resting {
		delete(b.orders, orderID)
		for _, set := range []*[]priceLevel{&b.bids, &b.asks} {
			for i := range *set {
				for j, ord := range (*set)[i].Orders {
					if ord.ID == orderID {
						(*set)[i].Orders = append((*set)[i].Orders[:j], (*set)[i].Orders[j+1:]...)
						return ExpectedFill{Status: StatusCancelled}
					}
				}
			}
		}
		return ExpectedFill{Status: StatusCancelled}
	}
	if b.filled[orderID] {
		return ExpectedFill{Status: StatusAlreadyFilled}
	}
	return ExpectedFill{Status: StatusNotFound}
}

// LiquidityFacts is an immutable snapshot of the ordering-independent facts the
// validator needs for hard-violation checks.
type LiquidityFacts struct {
	MinAskEver float64
	MaxBidEver float64
	AnyAsk     bool
	AnyBid     bool
}

// Facts returns the accumulated liquidity facts.
func (b *OrderBook) Facts() LiquidityFacts {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return LiquidityFacts{
		MinAskEver: b.minAskEver,
		MaxBidEver: b.maxBidEver,
		AnyAsk:     !math.IsInf(b.minAskEver, 1),
		AnyBid:     !math.IsInf(b.maxBidEver, -1),
	}
}

// PriceEverQuoted reports whether a price within one cent of the given price was
// ever quoted as a limit order. Fills can only occur at a resting limit price,
// so a reported fill at a never-quoted price is impossible.
func (b *OrderBook) PriceEverQuoted(price float64) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	c := priceCents(price)
	return b.seenPrices[c] || b.seenPrices[c-1] || b.seenPrices[c+1]
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
