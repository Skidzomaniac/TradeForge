// Package api exposes read-only historical query endpoints.
package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server serves historical metric queries from TimescaleDB.
type Server struct {
	pool      *pgxpool.Pool
	authToken string
}

// NewServer constructs the API server. authToken guards the metrics endpoints
// (presented as X-Internal-Token); /v1/health stays public for probes.
func NewServer(pool *pgxpool.Pool, authToken string) *Server {
	return &Server{pool: pool, authToken: authToken}
}

// Handler returns the configured mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", s.health)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/readyz", s.health)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/v1/metrics/", s.requireToken(s.metricsRouter))
	mux.HandleFunc("/v1/analysis/latency-distribution", s.requireToken(s.latencyDistribution))
	mux.HandleFunc("/v1/analysis/head-to-head", s.requireToken(s.headToHead))
	return mux
}

// requireToken enforces a constant-time internal-token check.
func (s *Server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Token")), []byte(s.authToken)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// metricsRouter handles /v1/metrics/{contestant_id}/latency etc.
func (s *Server) metricsRouter(w http.ResponseWriter, r *http.Request) {
	// path: /v1/metrics/{id}/{kind}
	rest := r.URL.Path[len("/v1/metrics/"):]
	var id, kind string
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' {
			id, kind = rest[:i], rest[i+1:]
			break
		}
	}
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing contestant_id"})
		return
	}
	switch kind {
	case "latency":
		s.latencyHistory(w, r, id)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown metric kind"})
	}
}

type latencyPoint struct {
	Bucket time.Time `json:"bucket"`
	P50    float64   `json:"p50"`
	P90    float64   `json:"p90"`
	P99    float64   `json:"p99"`
	Count  int64     `json:"order_count"`
}

func (s *Server) latencyHistory(w http.ResponseWriter, r *http.Request, contestantID string) {
	q := r.URL.Query()
	start := parseUnixMs(q.Get("start"), time.Now().Add(-time.Hour))
	end := parseUnixMs(q.Get("end"), time.Now())

	const sql = `
		SELECT time_bucket('1 second', time) AS bucket,
		       percentile_cont(0.50) WITHIN GROUP (ORDER BY latency_us) AS p50,
		       percentile_cont(0.90) WITHIN GROUP (ORDER BY latency_us) AS p90,
		       percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_us) AS p99,
		       COUNT(*) AS order_count
		FROM latency_samples
		WHERE contestant_id = $1 AND time >= $2 AND time <= $3
		GROUP BY bucket ORDER BY bucket ASC`
	rows, err := s.pool.Query(r.Context(), sql, contestantID, start, end)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	var out []latencyPoint
	for rows.Next() {
		var p latencyPoint
		if err := rows.Scan(&p.Bucket, &p.P50, &p.P90, &p.P99, &p.Count); err != nil {
			continue
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"contestant_id": contestantID, "points": out})
}

func parseUnixMs(s string, def time.Time) time.Time {
	if s == "" {
		return def
	}
	ms, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return time.UnixMilli(ms)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
