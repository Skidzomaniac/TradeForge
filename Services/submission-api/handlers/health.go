package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/trade-eval/submission-api/apierrors"
)

// Health handles GET /v1/health, pinging dependencies in parallel.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	var (
		mu     sync.Mutex
		states = map[string]string{}
		wg     sync.WaitGroup
	)
	set := func(name, state string) {
		mu.Lock()
		states[name] = state
		mu.Unlock()
	}

	wg.Add(3)
	go func() {
		defer wg.Done()
		if h.d.Redis == nil {
			set("redis", "disabled")
			return
		}
		if err := h.d.Redis.Ping(ctx).Err(); err != nil {
			set("redis", "down")
		} else {
			set("redis", "ok")
		}
	}()
	go func() {
		defer wg.Done()
		if h.d.DBPing == nil {
			set("db", "disabled")
			return
		}
		if err := h.d.DBPing(); err != nil {
			set("db", "down")
		} else {
			set("db", "ok")
		}
	}()
	go func() {
		defer wg.Done()
		// Kafka is producer-only here; report configured as ok.
		if h.d.Producer == nil {
			set("kafka", "disabled")
		} else {
			set("kafka", "ok")
		}
	}()
	wg.Wait()

	healthy := true
	for _, s := range states {
		if s == "down" {
			healthy = false
		}
	}

	status := http.StatusOK
	overall := "ok"
	if !healthy {
		status = http.StatusServiceUnavailable
		overall = "degraded"
	}

	body := map[string]any{
		"status":         overall,
		"version":        "1.0.0",
		"environment":    "dev",
		"uptime_seconds": time.Now().Unix() - h.d.StartedAt,
		"kafka":          states["kafka"],
		"redis":          states["redis"],
		"db":             states["db"],
	}
	apierrors.WriteJSON(w, status, body)
}
