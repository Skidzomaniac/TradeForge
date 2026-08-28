package api

import (
	"net/http"
	"time"
)

// latencyDistribution serves a histogram-bucketed latency distribution for a
// contestant+test, for the violin/heatmap charts.
func (s *Server) latencyDistribution(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	contestantID := q.Get("contestant_id")
	if contestantID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "contestant_id required"})
		return
	}
	const sql = `
		SELECT width_bucket(latency_us, 0, 10000, 10) AS bucket, COUNT(*) AS n
		FROM latency_samples
		WHERE contestant_id = $1 AND time > NOW() - INTERVAL '1 hour'
		GROUP BY bucket ORDER BY bucket`
	rows, err := s.pool.Query(r.Context(), sql, contestantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	type bucket struct {
		Bucket int   `json:"bucket"`
		Count  int64 `json:"count"`
	}
	var out []bucket
	for rows.Next() {
		var b bucket
		if rows.Scan(&b.Bucket, &b.Count) == nil {
			out = append(out, b)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"contestant_id": contestantID, "buckets": out})
}

// headToHead compares two contestants' best recent percentiles.
func (s *Server) headToHead(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	a, b := q.Get("contestant_a"), q.Get("contestant_b")
	if a == "" || b == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "contestant_a and contestant_b required"})
		return
	}
	type side struct {
		ContestantID string  `json:"contestant_id"`
		P99          float64 `json:"p99_us"`
		Orders       int64   `json:"orders"`
	}
	load := func(id string) side {
		const sql = `
			SELECT COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_us),0), COUNT(*)
			FROM latency_samples WHERE contestant_id = $1 AND time > NOW() - INTERVAL '1 hour'`
		var sd side
		sd.ContestantID = id
		_ = s.pool.QueryRow(r.Context(), sql, id).Scan(&sd.P99, &sd.Orders)
		return sd
	}
	sa, sb := load(a), load(b)
	winner := a
	if sb.P99 > 0 && sb.P99 < sa.P99 {
		winner = b
	}
	writeJSON(w, http.StatusOK, map[string]any{"a": sa, "b": sb, "winner": winner, "as_of": time.Now().UnixMilli()})
}
