package hub

import (
	"log/slog"
	"sync"
	"testing"
	"time"
)

type fakeBroadcaster struct {
	msgs [][]byte
	mu   sync.Mutex
}

func (f *fakeBroadcaster) record(msg []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, msg)
}

func (f *fakeBroadcaster) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.msgs)
}

func TestHub_SlowClientDropped(t *testing.T) {
	logger := slog.Default()
	h := NewHub(100, logger)
	go h.Run()

	slowSend := make(chan []byte, 1)
	slowClient := &Client{hub: h, send: slowSend, id: "slow"}
	h.register <- slowClient
	time.Sleep(20 * time.Millisecond)

	if h.Count() != 1 {
		t.Fatalf("expected 1 client, got %d", h.Count())
	}

	slowSend <- []byte("fill-buffer")

	for i := 0; i < 10; i++ {
		h.Broadcast([]byte(`{"type":"leaderboard_update"}`))
	}
	time.Sleep(50 * time.Millisecond)

	if h.Count() != 0 {
		t.Fatalf("slow client should have been dropped, got %d clients", h.Count())
	}
}

func TestHub_MaxClientsEnforced(t *testing.T) {
	logger := slog.Default()
	h := NewHub(2, logger)
	go h.Run()

	c1 := &Client{hub: h, send: make(chan []byte, 256), id: "c1"}
	c2 := &Client{hub: h, send: make(chan []byte, 256), id: "c2"}
	c3 := &Client{hub: h, send: make(chan []byte, 256), id: "c3"}

	h.register <- c1
	h.register <- c2
	time.Sleep(20 * time.Millisecond)

	if h.Count() != 2 {
		t.Fatalf("expected 2 clients, got %d", h.Count())
	}

	h.register <- c3
	time.Sleep(20 * time.Millisecond)

	if h.Count() != 2 {
		t.Fatalf("max clients exceeded: expected 2, got %d", h.Count())
	}
}

func TestHub_NewClientReceivesLastBroadcast(t *testing.T) {
	logger := slog.Default()
	h := NewHub(100, logger)
	go h.Run()

	h.Broadcast([]byte(`{"type":"snapshot"}`))
	time.Sleep(20 * time.Millisecond)

	recv := make(chan []byte, 256)
	c := &Client{hub: h, send: recv, id: "late"}
	h.register <- c
	time.Sleep(20 * time.Millisecond)

	select {
	case msg := <-recv:
		if string(msg) != `{"type":"snapshot"}` {
			t.Fatalf("unexpected message: %s", msg)
		}
	default:
		t.Fatal("new client did not receive the last broadcast")
	}
}

func TestHub_BroadcastDeliveredToAllClients(t *testing.T) {
	logger := slog.Default()
	h := NewHub(100, logger)
	go h.Run()

	clients := make([]*Client, 5)
	for i := range clients {
		clients[i] = &Client{hub: h, send: make(chan []byte, 256), id: "c"}
		h.register <- clients[i]
	}
	time.Sleep(20 * time.Millisecond)

	payload := []byte(`{"entries":[]}`)
	h.Broadcast(payload)
	time.Sleep(20 * time.Millisecond)

	for i, c := range clients {
		select {
		case msg := <-c.send:
			if string(msg) != string(payload) {
				t.Fatalf("client %d received wrong message", i)
			}
		default:
			t.Fatalf("client %d did not receive broadcast", i)
		}
	}
}

func TestHub_UnregisterCleansUp(t *testing.T) {
	logger := slog.Default()
	h := NewHub(100, logger)
	go h.Run()

	c := &Client{hub: h, send: make(chan []byte, 256), id: "cleanup"}
	h.register <- c
	time.Sleep(20 * time.Millisecond)

	if h.Count() != 1 {
		t.Fatalf("expected 1 client, got %d", h.Count())
	}

	h.unregister <- c
	time.Sleep(20 * time.Millisecond)

	if h.Count() != 0 {
		t.Fatalf("expected 0 clients after unregister, got %d", h.Count())
	}
}
