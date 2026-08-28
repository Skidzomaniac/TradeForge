package main

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/trade-eval/leaderboard-api/hub"
)

var wsConnections = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "leaderboard_websocket_connections",
	Help: "Currently connected WebSocket clients.",
})

// sampleWSConnections updates the gauge from the hub every 5s.
func sampleWSConnections(ctx context.Context, h *hub.Hub) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			wsConnections.Set(float64(h.Count()))
		}
	}
}
