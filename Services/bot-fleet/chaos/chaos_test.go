package chaos

import (
	"context"
	"testing"
	"time"

	"github.com/trade-eval/bot-fleet/bots"
	"github.com/trade-eval/bot-fleet/client"
)

func TestCircuitBreaker_OpensOnHighFailureRate(t *testing.T) {
	srv := NewControllableMockServer(0, 1.0) // always error
	defer srv.Kill()
	c := client.New(srv.URL(), bots.SharedTransport(), 2*time.Second)
	cb := client.NewCircuitBreaker()

	ctx := context.Background()
	for i := 0; i < 30; i++ {
		if !cb.Allow() {
			break
		}
		_, _, err := c.SubmitOrder(ctx, client.OrderRequest{OrderID: "x", Type: "MARKET_BUY", Quantity: 1})
		cb.Record(err == nil)
	}
	if cb.State() != client.Open {
		t.Fatalf("expected circuit OPEN after sustained failures, got %v", cb.State())
	}
}

func TestMockServer_HealthyFillsSucceed(t *testing.T) {
	srv := NewControllableMockServer(0, 0)
	defer srv.Kill()
	c := client.New(srv.URL(), bots.SharedTransport(), 2*time.Second)
	resp, _, err := c.SubmitOrder(context.Background(), client.OrderRequest{OrderID: "y", Type: "MARKET_BUY", Quantity: 1})
	if err != nil || resp.Status != "FILLED" {
		t.Fatalf("expected FILLED, got %+v err=%v", resp, err)
	}
	if srv.OrdersReceived() != 1 {
		t.Fatalf("expected 1 order received, got %d", srv.OrdersReceived())
	}
}

func TestSlowServer_TimeoutRecorded(t *testing.T) {
	srv := NewControllableMockServer(200*time.Millisecond, 0)
	defer srv.Kill()
	c := client.New(srv.URL(), bots.SharedTransport(), 50*time.Millisecond) // tighter than delay
	_, _, err := c.SubmitOrder(context.Background(), client.OrderRequest{OrderID: "z", Type: "MARKET_BUY", Quantity: 1})
	if err != client.ErrTimedOut {
		t.Fatalf("expected timeout, got %v", err)
	}
}
