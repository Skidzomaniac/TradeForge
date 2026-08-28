// Package chaos provides a controllable mock order book server and resilience
// tests for the bot fleet.
package chaos

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"time"
)

// ControllableMockServer is an order book server whose latency and error rate
// can be mutated mid-test, and which can be killed and restarted.
type ControllableMockServer struct {
	mu        sync.RWMutex
	delay     time.Duration
	errorRate float64
	received  atomic.Int64
	server    *httptest.Server
	handler   http.Handler
}

// NewControllableMockServer starts a mock server with the given delay and error rate.
func NewControllableMockServer(delay time.Duration, errorRate float64) *ControllableMockServer {
	m := &ControllableMockServer{delay: delay, errorRate: errorRate}
	m.handler = http.HandlerFunc(m.serve)
	m.server = httptest.NewServer(m.handler)
	return m
}

// URL returns the server base URL.
func (m *ControllableMockServer) URL() string { return m.server.URL }

// SetDelay changes the response delay.
func (m *ControllableMockServer) SetDelay(d time.Duration) {
	m.mu.Lock()
	m.delay = d
	m.mu.Unlock()
}

// SetErrorRate changes the fraction of 500 responses.
func (m *ControllableMockServer) SetErrorRate(r float64) {
	m.mu.Lock()
	m.errorRate = r
	m.mu.Unlock()
}

// OrdersReceived returns how many order requests were handled.
func (m *ControllableMockServer) OrdersReceived() int64 { return m.received.Load() }

// Kill stops the server.
func (m *ControllableMockServer) Kill() { m.server.Close() }

// Restart starts a fresh server (new URL).
func (m *ControllableMockServer) Restart() { m.server = httptest.NewServer(m.handler) }

func (m *ControllableMockServer) serve(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	delay, errRate := m.delay, m.errorRate
	m.mu.RUnlock()

	if delay > 0 {
		time.Sleep(delay)
	}
	if r.URL.Path == "/health" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		return
	}
	m.received.Add(1)
	if errRate > 0 && rand.Float64() < errRate {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"FILLED","filled_price":100,"filled_quantity":1,"remaining_quantity":0}`))
}
