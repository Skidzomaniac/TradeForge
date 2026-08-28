package bots

import (
	"context"
	"fmt"
	"time"

	"github.com/trade-eval/bot-fleet/client"
)

// RunCorrectnessPhase drives the contestant with a single fully serialized order
// stream: one logical sender, one in-flight request at a time, monotonic
// send-time sequence numbers. Because the platform issues each order only after
// the previous one is acknowledged, the true processing order equals the send
// order, so the reference replay in sequence order is exact. This phase produces
// the correctness_rate. It must run to completion before any concurrent load bot
// starts, so that every correctness event carries a lower sequence number than
// every load event and the ingester's reference book processes the serialized
// stream first.
//
// The script deliberately exercises every reachable outcome: resting limit
// orders (PENDING), crossing orders (FILLED), multi-level and partial fills
// (PARTIAL), market orders with no liquidity (REJECTED), a cancel of a resting
// order (CANCELLED), and a cancel of an already-removed order (NOT_FOUND). A
// submission that fakes fills disagrees with the reference on the PENDING,
// REJECTED, and CANCELLED steps and is also caught by the hard-violation checks.
func RunCorrectnessPhase(ctx context.Context, botID string, d Deps) {
	const persona = "correctness"
	step := 0
	nextID := func() string { step++; return fmt.Sprintf("corr_%s_%d", d.TestID, step) }

	submit := func(typ string, price, qty float64) string {
		if ctx.Err() != nil {
			return ""
		}
		oid := nextID()
		seq := d.nextSeq()
		sent := time.Now().UnixNano()
		resp, _, err := d.Client.SubmitOrder(ctx, client.OrderRequest{OrderID: oid, Type: typ, Price: price, Quantity: qty})
		acked := time.Now().UnixNano()
		// The ingester recomputes correctness from the reference book; the bot's
		// self-reported flag is not used for scoring, so a placeholder is fine.
		d.measureAndEmit(seq, PhaseCorrectness, oid, botID, persona, typ, sent, acked, price, qty,
			ExpectedFill{}, resp.Status, resp.FilledPrice, resp.FilledQuantity, err == nil, err == client.ErrTimedOut)
		return oid
	}
	cancel := func(oid string) {
		if ctx.Err() != nil || oid == "" {
			return
		}
		seq := d.nextSeq()
		sent := time.Now().UnixNano()
		resp, _, err := d.Client.CancelOrder(ctx, oid)
		acked := time.Now().UnixNano()
		d.measureAndEmit(seq, PhaseCorrectness, oid, botID, persona, "CANCEL", sent, acked, 0, 0,
			ExpectedFill{}, resp.Status, 0, 0, err == nil, err == client.ErrTimedOut)
	}

	// Rest liquidity on both sides without crossing.
	submit("LIMIT_SELL", 101, 10) // ask, rests -> PENDING
	submit("LIMIT_SELL", 102, 10) // ask, rests -> PENDING
	bid := submit("LIMIT_BUY", 99, 10) // bid, rests -> PENDING

	// Cross the inside ask partially, then sweep the rest with a market order.
	submit("LIMIT_BUY", 101, 5) // crosses ask@101 -> FILLED 5 @ 101
	submit("MARKET_BUY", 0, 5)  // consumes remaining 5 of ask@101 -> FILLED @ 101

	// Multi-level partial: only 10 left at 102, ask side then empty.
	submit("MARKET_BUY", 0, 15) // fills 10 @ 102, 5 unfilled -> PARTIAL
	submit("MARKET_BUY", 0, 5)  // no asks left -> REJECTED

	// Cancel the resting bid, then cancel it again.
	cancel(bid)               // -> CANCELLED
	cancel(bid)               // already removed -> NOT_FOUND
	submit("MARKET_SELL", 0, 5) // no bids left -> REJECTED

	// Cross a fresh resting ask with a larger limit buy (partial rest).
	submit("LIMIT_SELL", 100, 20) // rests -> PENDING
	rest := submit("LIMIT_BUY", 100, 30) // fills 20 @ 100, 10 rests -> PARTIAL
	cancel(rest)                          // cancels the resting remainder -> CANCELLED
}
