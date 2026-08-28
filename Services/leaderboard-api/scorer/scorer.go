// Package scorer computes the ranked leaderboard from Redis metrics.
package scorer

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// MinValidOrders is the minimum number of valid (non-timed-out) orders a
// contestant must submit to be ranked. Below it the contestant is disqualified
// and listed after every ranked entry.
const MinValidOrders = 100

// Entry is one ranked contestant row.
type Entry struct {
	Rank            int     `json:"rank"`
	ContestantID    string  `json:"contestant_id"`
	ContestantName  string  `json:"contestant_name"`
	Score           float64 `json:"score"`
	P50Us           int64   `json:"p50_us"`
	P90Us           int64   `json:"p90_us"`
	P99Us           int64   `json:"p99_us"`
	TPS             float64 `json:"tps"`
	CorrectnessRate float64 `json:"correctness_rate"`
	ValidOrders     int64   `json:"valid_orders"`
	Disqualified    bool    `json:"disqualified"`
	Status          string  `json:"status"`
	LastUpdatedNs   int64   `json:"last_updated_ns"`
}

// Update is the broadcast payload.
type Update struct {
	Type      string  `json:"type"`
	Timestamp int64   `json:"timestamp"`
	Entries   []Entry `json:"entries"`
}

// Broadcaster receives leaderboard updates.
type Broadcaster interface{ Broadcast([]byte) }

// CommentaryFn turns successive leaderboards into serialized ticker events.
type CommentaryFn func(entries []Entry) [][]byte

// Scorer periodically recomputes and broadcasts the leaderboard.
type Scorer struct {
	redis      *redis.Client
	hub        Broadcaster
	interval   time.Duration
	logger     *slog.Logger
	commentary CommentaryFn
	predictor  *Predictor
}

// New constructs the scorer.
func New(rdb *redis.Client, hub Broadcaster, interval time.Duration, logger *slog.Logger) *Scorer {
	return &Scorer{redis: rdb, hub: hub, interval: interval, logger: logger}
}

// SetCommentary attaches a commentary generator whose events are published
// alongside leaderboard updates.
func (s *Scorer) SetCommentary(fn CommentaryFn) { s.commentary = fn }

// SetPredictor attaches a score predictor fed each scoring cycle.
func (s *Scorer) SetPredictor(p *Predictor) { s.predictor = p }

// GetPrediction returns the projected outcome for a contestant.
func (s *Scorer) GetPrediction(contestantID string) Prediction {
	if s.predictor == nil {
		return Prediction{ContestantID: contestantID}
	}
	return s.predictor.Predict(contestantID)
}

// Run recomputes + broadcasts on a ticker until ctx is done.
func (s *Scorer) Run(ctx context.Context) {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.isFrozen(ctx) {
				continue
			}
			entries, err := s.Compute(ctx)
			if err != nil {
				continue
			}
			if s.predictor != nil {
				s.predictor.Record(entries)
			}
			upd := Update{Type: "leaderboard_update", Timestamp: time.Now().UnixMilli(), Entries: entries}
			if b, err := json.Marshal(upd); err == nil {
				// Publish to Redis only; every pod's hub (including this one)
				// receives it via the pub/sub subscriber, so we never
				// double-broadcast on the scorer's own pod.
				s.redis.Set(ctx, "leaderboard:cached", b, 2*time.Second)
				s.redis.Publish(ctx, "leaderboard:updates", b)
			}
			if s.commentary != nil {
				for _, evt := range s.commentary(entries) {
					s.redis.Publish(ctx, "leaderboard:updates", evt)
					s.redis.LPush(ctx, "ticker:events", evt)
					s.redis.LTrim(ctx, "ticker:events", 0, 49)
				}
			}
		}
	}
}

func (s *Scorer) isFrozen(ctx context.Context) bool {
	v, _ := s.redis.Exists(ctx, "leaderboard:frozen").Result()
	return v == 1
}

// Compute reads metrics for all active contestants and returns ranked entries.
func (s *Scorer) Compute(ctx context.Context) ([]Entry, error) {
	ids, err := s.redis.SMembers(ctx, "leaderboard:active_contestants").Result()
	if err != nil {
		return nil, err
	}
	pipe := s.redis.Pipeline()
	cmds := make(map[string]*redis.MapStringStringCmd, len(ids))
	for _, id := range ids {
		cmds[id] = pipe.HGetAll(ctx, "metrics:"+id)
	}
	_, _ = pipe.Exec(ctx)

	var entries []Entry
	for id, cmd := range cmds {
		m := cmd.Val()
		if len(m) == 0 {
			continue
		}
		entries = append(entries, Entry{
			ContestantID:    id,
			ContestantName:  orDefault(m["contestant_name"], id),
			P50Us:           atoiI64(m["p50_latency_us"]),
			P90Us:           atoiI64(m["p90_latency_us"]),
			P99Us:           atoiI64(m["p99_latency_us"]),
			TPS:             atof(m["tps"]),
			CorrectnessRate: atof(m["correctness_rate"]),
			ValidOrders:     atoiI64(m["valid_orders"]),
			Status:          orDefault(m["test_status"], "idle"),
			LastUpdatedNs:   atoiI64(m["last_updated_ns"]),
		})
	}

	return RankEntries(entries), nil
}

// RankEntries scores and orders entries, applying the disqualification rule.
// Contestants below MinValidOrders are scored 0, marked disqualified, and listed
// after every ranked contestant. Scoring normalizes only over qualified entries
// so a disqualified outlier cannot skew the qualified field.
func RankEntries(entries []Entry) []Entry {
	qualified, disqualified := make([]Entry, 0, len(entries)), make([]Entry, 0)
	for _, e := range entries {
		if e.ValidOrders >= MinValidOrders {
			qualified = append(qualified, e)
		} else {
			e.Disqualified = true
			e.Status = "disqualified"
			e.Score = 0
			disqualified = append(disqualified, e)
		}
	}

	ComputeScores(qualified)
	sort.SliceStable(qualified, func(i, j int) bool {
		if qualified[i].Score != qualified[j].Score {
			return qualified[i].Score > qualified[j].Score
		}
		if qualified[i].CorrectnessRate != qualified[j].CorrectnessRate {
			return qualified[i].CorrectnessRate > qualified[j].CorrectnessRate
		}
		return qualified[i].P99Us < qualified[j].P99Us
	})
	rank := 1
	for i := range qualified {
		if i > 0 && qualified[i-1].Score != qualified[i].Score {
			rank = i + 1
		}
		qualified[i].Rank = rank
	}

	sort.SliceStable(disqualified, func(i, j int) bool {
		return disqualified[i].ContestantID < disqualified[j].ContestantID
	})
	return append(qualified, disqualified...)
}

// ComputeScores assigns composite scores in-place via min-max normalization.
func ComputeScores(entries []Entry) {
	if len(entries) == 0 {
		return
	}
	minTPS, maxTPS := entries[0].TPS, entries[0].TPS
	minP99, maxP99 := float64(entries[0].P99Us), float64(entries[0].P99Us)
	for _, e := range entries {
		p99 := float64(e.P99Us)
		minTPS, maxTPS = minF(minTPS, e.TPS), maxF(maxTPS, e.TPS)
		minP99, maxP99 = minF(minP99, p99), maxF(maxP99, p99)
	}
	for i := range entries {
		nt := norm(entries[i].TPS, minTPS, maxTPS)
		np := norm(float64(entries[i].P99Us), minP99, maxP99)
		score := 0.40*nt + 0.40*(1-np) + 0.20*entries[i].CorrectnessRate
		entries[i].Score = round2(clamp(score*100, 0, 100))
	}
}

func norm(v, min, max float64) float64 {
	if max == min {
		return 1.0
	}
	return (v - min) / (max - min)
}
func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
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
func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
func atoiI64(s string) int64   { v, _ := strconv.ParseInt(s, 10, 64); return v }
func atof(s string) float64    { v, _ := strconv.ParseFloat(s, 64); return v }
func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
