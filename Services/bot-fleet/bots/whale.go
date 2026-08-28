package bots

import (
	"context"
	"time"

	"github.com/trade-eval/bot-fleet/client"
)

func runWhale(ctx context.Context, botID string, d Deps) {
	cycle := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter(500*time.Millisecond, 0.1)):
		}
		if !d.Breaker.Allow() {
			continue
		}
		cycle++

		const qty = 10000.0
		var typ, subtype string
		var price float64
		if cycle%5 == 0 {
			typ, subtype = "MARKET_BUY", "whale_market"
		} else {
			typ, subtype, price = "LIMIT_BUY", "whale_limit", 100.0
		}

		oid := generateOrderID()
		seq := d.nextSeq()
		sent := time.Now().UnixNano()
		req := client.OrderRequest{OrderID: oid, Type: typ, Quantity: qty}
		if price > 0 {
			req.Price = price
		}
		resp, _, err := d.Client.SubmitOrder(ctx, req)
		acked := time.Now().UnixNano()
		timedOut := err == client.ErrTimedOut
		d.Breaker.Record(err == nil)

		// Partial/pending are expected for whale orders; validate sanity only.
		correct := resp.FilledQuantity >= 0 && resp.FilledQuantity <= qty+1e-9
		expected := ExpectedFill{Status: resp.Status, Quantity: qty}
		_ = subtype
		d.measureAndEmit(seq, PhaseLoad, oid, botID, "whale", typ, sent, acked, price, qty, expected, resp.Status, resp.FilledPrice, resp.FilledQuantity, correct && !timedOut, timedOut)
	}
}
