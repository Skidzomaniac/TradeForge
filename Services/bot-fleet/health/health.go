// Package health serves liveness/readiness endpoints.
package health

import (
	"net/http"
	"sync/atomic"
)

// Server tracks readiness and serves probes plus /metrics.
type Server struct {
	ready atomic.Bool
}

// NewServer constructs a health server.
func NewServer() *Server { return &Server{} }

// SetReady marks the service ready (Kafka consumer connected).
func (s *Server) SetReady(v bool) { s.ready.Store(v) }

// Handler returns a mux with /healthz, /readyz and the supplied /metrics handler.
func (s *Server) Handler(metricsHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if s.ready.Load() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	if metricsHandler != nil {
		mux.Handle("/metrics", metricsHandler)
	}
	return mux
}
