package bots

import (
	"context"
	"time"

	"github.com/trade-eval/bot-fleet/client"
)

func runAggressiveTaker(ctx context.Context, botID string, d Deps) {
	buy := true
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter(50*time.Millisecond, 0.2)):
		}
		if !d.Breaker.Allow() {
			continue
		}

		typ := "MARKET_BUY"
		if !buy {
			typ = "MARKET_SELL"
		}
		buy = !buy
		qty := randFloat(10, 100)

		oid := generateOrderID()
		seq := d.nextSeq()
		sent := time.Now().UnixNano()
		resp, _, err := d.Client.SubmitOrder(ctx, client.OrderRequest{OrderID: oid, Type: typ, Quantity: qty})
		acked := time.Now().UnixNano()
		timedOut := err == client.ErrTimedOut
		d.Breaker.Record(err == nil)

		// Market orders with liquidity should never be PENDING/REJECTED.
		correct := resp.Status == "FILLED" || resp.Status == "PARTIAL"
		expected := ExpectedFill{Status: "FILLED", Quantity: qty}
		d.measureAndEmit(seq, PhaseLoad, oid, botID, "aggressive_taker", typ, sent, acked, 0, qty, expected, resp.Status, resp.FilledPrice, resp.FilledQuantity, correct && !timedOut, timedOut)
	}
}
