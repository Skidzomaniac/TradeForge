// Package api serves the REST endpoints for the leaderboard service.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/trade-eval/leaderboard-api/scorer"
)

// Handlers serves leaderboard REST endpoints.
type Handlers struct {
	redis  *redis.Client
	scorer *scorer.Scorer
}

// New constructs the handler set.
func New(rdb *redis.Client, sc *scorer.Scorer) *Handlers {
	return &Handlers{redis: rdb, scorer: sc}
}

// GetLeaderboard serves GET /v1/leaderboard from cache or live compute.
func (h *Handlers) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	if cached, err := h.redis.Get(r.Context(), "leaderboard:cached").Bytes(); err == nil {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(cached)
		return
	}
	entries, err := h.scorer.Compute(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated_at": time.Now().UnixMilli(), "entries": entries})
}

// GetAnomalies serves GET /v1/contestants/{id}/anomalies.
func (h *Handlers) GetAnomalies(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/contestants/"), "/anomalies")
	vals, _ := h.redis.LRange(r.Context(), "anomalies:"+id, 0, 99).Result()
	out := make([]json.RawMessage, 0, len(vals))
	for _, v := range vals {
		out = append(out, json.RawMessage(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"contestant_id": id, "anomalies": out})
}

// Health serves GET /v1/health.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	status := "ok"
	code := http.StatusOK
	if h.redis.Ping(ctx).Err() != nil {
		status, code = "degraded", http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]string{"status": status})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
