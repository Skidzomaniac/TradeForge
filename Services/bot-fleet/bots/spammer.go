package bots

import (
	"context"
	"time"

	"github.com/trade-eval/bot-fleet/client"
)

func runSpammer(ctx context.Context, botID string, d Deps) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(jitter(time.Millisecond, 0.5)):
		}
		if !d.Breaker.Allow() {
			continue
		}

		oid := generateOrderID()
		price := round2(randFloat(40, 60)) // far from mid (~100), unlikely to fill

		// 1) submit the resting limit order
		seq := d.nextSeq()
		sent := time.Now().UnixNano()
		resp, _, err := d.Client.SubmitOrder(ctx, client.OrderRequest{OrderID: oid, Type: "LIMIT_BUY", Price: price, Quantity: 1})
		acked := time.Now().UnixNano()
		timedOut := err == client.ErrTimedOut
		d.Breaker.Record(err == nil)
		expected := ExpectedFill{Status: "PENDING"}
		correct := resp.Status == "PENDING" || resp.Status == "FILLED" || resp.Status == "PARTIAL"
		d.measureAndEmit(seq, PhaseLoad, oid, botID, "spammer", "LIMIT_BUY", sent, acked, price, 1, expected, resp.Status, resp.FilledPrice, resp.FilledQuantity, correct && !timedOut, timedOut)

		// 2) immediately cancel using the same order ID
		cseq := d.nextSeq()
		csent := time.Now().UnixNano()
		cresp, _, cerr := d.Client.CancelOrder(ctx, oid)
		cacked := time.Now().UnixNano()
		ctimedOut := cerr == client.ErrTimedOut
		d.Breaker.Record(cerr == nil)
		ccorrect := cresp.Status == "CANCELLED" || cresp.Status == "FILLED" || cresp.Status == "NOT_FOUND"
		d.measureAndEmit(cseq, PhaseLoad, oid, botID, "spammer", "CANCEL", csent, cacked, 0, 0, ExpectedFill{Status: "CANCELLED"}, cresp.Status, 0, 0, ccorrect && !ctimedOut, ctimedOut)
	}
}
