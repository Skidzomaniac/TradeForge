// Package admin provides privileged contest-management endpoints.
package admin

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// Handlers serves admin endpoints guarded by a constant-time key check.
type Handlers struct {
	redis  *redis.Client
	apiKey string
}

// New constructs the admin handler set.
func New(rdb *redis.Client, apiKey string) *Handlers {
	return &Handlers{redis: rdb, apiKey: apiKey}
}

// Auth wraps a handler with constant-time admin-key verification.
func (h *Handlers) Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Admin-Key")
		if subtle.ConstantTimeCompare([]byte(key), []byte(h.apiKey)) != 1 {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// Freeze halts leaderboard updates (24h TTL).
func (h *Handlers) Freeze(w http.ResponseWriter, r *http.Request) {
	h.redis.Set(r.Context(), "leaderboard:frozen", "1", 24*time.Hour)
	writeJSON(w, http.StatusOK, map[string]string{"status": "frozen"})
}

// Unfreeze resumes leaderboard updates.
func (h *Handlers) Unfreeze(w http.ResponseWriter, r *http.Request) {
	h.redis.Del(r.Context(), "leaderboard:frozen")
	writeJSON(w, http.StatusOK, map[string]string{"status": "unfrozen"})
}

// SystemStatus reports a coarse platform health snapshot for the ops console.
func (h *Handlers) SystemStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	redisOK := h.redis.Ping(ctx).Err() == nil
	active, _ := h.redis.SCard(ctx, "leaderboard:active_contestants").Result()
	var mem string
	if info, err := h.redis.Info(ctx, "memory").Result(); err == nil {
		for _, line := range splitLines(info) {
			if len(line) > 18 && line[:18] == "used_memory_human:" {
				mem = line[18:]
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"redis":                map[string]any{"healthy": redisOK, "memory": mem},
		"active_contestants":   active,
		"leaderboard_frozen":   h.redis.Exists(ctx, "leaderboard:frozen").Val() == 1,
	})
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// Disqualify removes a contestant from the active set.
func (h *Handlers) Disqualify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ContestantID string `json:"contestant_id"`
		Reason       string `json:"reason"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.ContestantID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "contestant_id required"})
		return
	}
	h.redis.Set(r.Context(), "disqualified:"+body.ContestantID, body.Reason, 0)
	h.redis.SRem(r.Context(), "leaderboard:active_contestants", body.ContestantID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "disqualified", "contestant_id": body.ContestantID})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
