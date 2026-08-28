package api

import (
	"net/http"
	"strings"
)

// GetInsights serves GET /v1/contestants/{id}/insights - current standing,
// weakest dimension vs peers, and auto-generated improvement tips.
func (h *Handlers) GetInsights(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/contestants/"), "/insights")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "contestant id required"})
		return
	}
	entries, err := h.scorer.Compute(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var me *insightEntry
	n := len(entries)
	tpsRankBetter, latRankBetter := 0, 0
	for _, e := range entries {
		if e.ContestantID == id {
			me = &insightEntry{Rank: e.Rank, Score: e.Score, TPS: e.TPS, P99: e.P99Us, Correctness: e.CorrectnessRate}
		}
	}
	if me == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "contestant not on leaderboard"})
		return
	}
	for _, e := range entries {
		if e.ContestantID == id {
			continue
		}
		if e.TPS > me.TPS {
			tpsRankBetter++
		}
		if e.P99Us > 0 && (me.P99 == 0 || e.P99Us < me.P99) {
			latRankBetter++
		}
	}

	tips := []string{}
	weakness := "balanced"
	if me.Correctness < 0.99 {
		tips = append(tips, "Correctness below 99% - check partial-fill and price-time-priority handling.")
		weakness = "correctness"
	}
	if tpsRankBetter > n/2 {
		tips = append(tips, "Throughput is below the median - consider lock-free data structures.")
		weakness = "throughput"
	}
	if latRankBetter > n/2 {
		tips = append(tips, "Latency is below the median - reduce per-order allocations and lock contention.")
		weakness = "latency"
	}
	if len(tips) == 0 {
		tips = append(tips, "Strong all-round - shave p99 tails to climb further.")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"contestant_id": id,
		"standing":      me,
		"weakness":      weakness,
		"tips":          tips,
		"field_size":    n,
	})
}

// GetPrediction serves GET /v1/contestants/{id}/prediction.
func (h *Handlers) GetPrediction(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/contestants/"), "/prediction")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "contestant id required"})
		return
	}
	writeJSON(w, http.StatusOK, h.scorer.GetPrediction(id))
}

type insightEntry struct {
	Rank        int     `json:"rank"`
	Score       float64 `json:"score"`
	TPS         float64 `json:"tps"`
	P99         int64   `json:"p99_us"`
	Correctness float64 `json:"correctness_rate"`
}
