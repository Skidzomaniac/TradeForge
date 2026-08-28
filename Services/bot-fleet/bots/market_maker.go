package bots

import (
	"context"
	"time"

	"github.com/trade-eval/bot-fleet/client"
)

func runMarketMaker(ctx context.Context, botID string, d Deps) {
	book := NewShadowBook()
	mid := 100.0
	spreadBPS := 10.0
	var buyID, sellID string
	var buyAt, sellAt time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter(10*time.Millisecond, 0.3)):
		}
		if !d.Breaker.Allow() {
			continue
		}

		mid *= 1 + randFloat(-0.0005, 0.0005)
		size := randFloat(1, 20)
		bid := round2(mid * (1 - spreadBPS/20000))
		ask := round2(mid * (1 + spreadBPS/20000))

		if buyID == "" {
			buyID = d.sendLimit(ctx, botID, "market_maker", "LIMIT_BUY", bid, size, book)
			buyAt = time.Now()
		}
		if sellID == "" {
			sellID = d.sendLimit(ctx, botID, "market_maker", "LIMIT_SELL", ask, size, book)
			sellAt = time.Now()
		}
		if buyID != "" && time.Since(buyAt) > 200*time.Millisecond {
			d.sendCancel(ctx, botID, "market_maker", buyID, book)
			buyID = ""
		}
		if sellID != "" && time.Since(sellAt) > 200*time.Millisecond {
			d.sendCancel(ctx, botID, "market_maker", sellID, book)
			sellID = ""
		}
	}
}

// sendLimit submits a limit order, emits telemetry, and returns the order id.
func (d Deps) sendLimit(ctx context.Context, botID, persona, typ string, price, qty float64, book *ShadowBook) string {
	oid := generateOrderID()
	expected := book.AddOrder(Order{OrderID: oid, Type: typ, Price: price, Quantity: qty, Time: time.Now()})
	seq := d.nextSeq()
	sent := time.Now().UnixNano()
	resp, _, err := d.Client.SubmitOrder(ctx, client.OrderRequest{OrderID: oid, Type: typ, Price: price, Quantity: qty})
	acked := time.Now().UnixNano()
	timedOut := err == client.ErrTimedOut
	d.Breaker.Record(err == nil)
	correct, _ := ValidateFill(expected, resp.Status, resp.FilledPrice, resp.FilledQuantity)
	d.measureAndEmit(seq, PhaseLoad, oid, botID, persona, typ, sent, acked, price, qty, expected, resp.Status, resp.FilledPrice, resp.FilledQuantity, correct && !timedOut, timedOut)
	return oid
}

// sendCancel cancels an order and emits telemetry.
func (d Deps) sendCancel(ctx context.Context, botID, persona, orderID string, book *ShadowBook) {
	book.AddOrder(Order{OrderID: orderID, Type: "CANCEL"})
	seq := d.nextSeq()
	sent := time.Now().UnixNano()
	resp, _, err := d.Client.CancelOrder(ctx, orderID)
	acked := time.Now().UnixNano()
	timedOut := err == client.ErrTimedOut
	d.Breaker.Record(err == nil)
	correct := resp.Status == "CANCELLED" || resp.Status == "NOT_FOUND" || resp.Status == "ALREADY_FILLED"
	expected := ExpectedFill{Status: "CANCELLED"}
	d.measureAndEmit(seq, PhaseLoad, orderID, botID, persona, "CANCEL", sent, acked, 0, 0, expected, resp.Status, 0, 0, correct && !timedOut, timedOut)
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
