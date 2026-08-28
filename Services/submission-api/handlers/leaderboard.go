package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type leaderboardEntry struct {
	Rank            int     `json:"rank"`
	ContestantID    string  `json:"contestant_id"`
	ContestantName  string  `json:"contestant_name"`
	Score           float64 `json:"score"`
	P50Us           int64   `json:"p50_us"`
	P90Us           int64   `json:"p90_us"`
	P99Us           int64   `json:"p99_us"`
	TPS             float64 `json:"tps"`
	CorrectnessRate float64 `json:"correctness_rate"`
	Status          string  `json:"status"`
	LastUpdatedNs   int64   `json:"last_updated_ns"`
}

type leaderboardResponse struct {
	UpdatedAt int64              `json:"updated_at"`
	Entries   []leaderboardEntry `json:"entries"`
}

// GetLeaderboard handles GET /v1/leaderboard (public, 500ms cached).
func (h *Handlers) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.d.Redis != nil {
		if cached, err := h.d.Redis.Get(ctx, "leaderboard:cached").Bytes(); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cached)
			return
		}
	}

	resp := h.computeLeaderboard(ctx)
	body, _ := json.Marshal(resp)
	if h.d.Redis != nil {
		h.d.Redis.Set(ctx, "leaderboard:cached", body, 500*time.Millisecond)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handlers) computeLeaderboard(ctx context.Context) leaderboardResponse {
	resp := leaderboardResponse{UpdatedAt: time.Now().UnixMilli(), Entries: []leaderboardEntry{}}
	if h.d.Redis == nil {
		return resp
	}

	ids, err := h.d.Redis.SMembers(ctx, "leaderboard:active_contestants").Result()
	if err != nil || len(ids) == 0 {
		return resp
	}

	var entries []leaderboardEntry
	for _, id := range ids {
		m, err := h.d.Redis.HGetAll(ctx, "metrics:"+id).Result()
		if err != nil || len(m) == 0 {
			continue
		}
		entries = append(entries, leaderboardEntry{
			ContestantID:    id,
			ContestantName:  m["contestant_name"],
			P50Us:           atoiI64(m["p50_latency_us"]),
			P90Us:           atoiI64(m["p90_latency_us"]),
			P99Us:           atoiI64(m["p99_latency_us"]),
			TPS:             atof(m["tps"]),
			CorrectnessRate: atof(m["correctness_rate"]),
			Status:          orDefault(m["test_status"], "idle"),
			LastUpdatedNs:   atoiI64(m["last_updated_ns"]),
		})
	}

	ComputeScores(entries)

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Score != entries[j].Score {
			return entries[i].Score > entries[j].Score
		}
		if entries[i].CorrectnessRate != entries[j].CorrectnessRate {
			return entries[i].CorrectnessRate > entries[j].CorrectnessRate
		}
		return entries[i].P99Us < entries[j].P99Us
	})

	rank := 1
	for i := range entries {
		if i > 0 && entries[i-1].Score != entries[i].Score {
			rank = i + 1
		}
		entries[i].Rank = rank
	}

	resp.Entries = entries
	return resp
}

// ComputeScores assigns composite scores in-place using min-max normalization
// across the provided entries:
//
//	score = 0.40*norm(tps) + 0.40*(1-norm(p99)) + 0.20*correctness, scaled to 0-100.
func ComputeScores(entries []leaderboardEntry) {
	if len(entries) == 0 {
		return
	}
	minTPS, maxTPS := entries[0].TPS, entries[0].TPS
	minP99, maxP99 := float64(entries[0].P99Us), float64(entries[0].P99Us)
	for _, e := range entries {
		p99 := float64(e.P99Us)
		if e.TPS < minTPS {
			minTPS = e.TPS
		}
		if e.TPS > maxTPS {
			maxTPS = e.TPS
		}
		if p99 < minP99 {
			minP99 = p99
		}
		if p99 > maxP99 {
			maxP99 = p99
		}
	}
	for i := range entries {
		nt := normalize(entries[i].TPS, minTPS, maxTPS)
		np := normalize(float64(entries[i].P99Us), minP99, maxP99)
		score := 0.40*nt + 0.40*(1.0-np) + 0.20*entries[i].CorrectnessRate
		score = clamp(score*100, 0, 100)
		entries[i].Score = round2(score)
	}
}

func normalize(v, min, max float64) float64 {
	if max == min {
		return 1.0
	}
	return (v - min) / (max - min)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func atoiI64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func atof(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
