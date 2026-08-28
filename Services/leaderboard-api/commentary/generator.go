// Package commentary turns leaderboard deltas into human-readable ticker events.
package commentary

import (
	"time"

	"github.com/trade-eval/leaderboard-api/scorer"
)

// Event is one ticker line.
type Event struct {
	Type      string `json:"type"` // always "ticker_event"
	Message   string `json:"message"`
	Priority  int    `json:"priority"`
	CreatedAt int64  `json:"created_at"`
}

// Generator compares successive leaderboards and emits commentary.
type Generator struct {
	prevRank  map[string]int
	prevScore map[string]float64
}

// New constructs the generator.
func New() *Generator {
	return &Generator{prevRank: map[string]int{}, prevScore: map[string]float64{}}
}

// Diff returns commentary for the transition to the given entries, then updates
// internal state.
func (g *Generator) Diff(entries []scorer.Entry) []Event {
	now := time.Now().UnixMilli()
	var out []Event
	for _, e := range entries {
		prev, seen := g.prevRank[e.ContestantID]
		switch {
		case !seen && e.Score > 0:
			out = append(out, Event{"ticker_event", "NEW: " + e.ContestantName + " enters the board at #" + itoa(e.Rank), 2, now})
		case seen && e.Rank < prev-2:
			out = append(out, Event{"ticker_event", "+" + itoa(prev-e.Rank) + ": " + e.ContestantName + " climbs to #" + itoa(e.Rank), 3, now})
		case seen && e.Rank < prev:
			out = append(out, Event{"ticker_event", "▲ " + e.ContestantName + " moves up to #" + itoa(e.Rank), 1, now})
		case seen && e.Rank > prev:
			out = append(out, Event{"ticker_event", "▼ " + e.ContestantName + " drops to #" + itoa(e.Rank), 1, now})
		}
		if e.TPS >= 10000 && g.prevScore[e.ContestantID] < e.Score {
			out = append(out, Event{"ticker_event", e.ContestantName + " clears 10,000 orders/sec", 2, now})
		}
		g.prevRank[e.ContestantID] = e.Rank
		g.prevScore[e.ContestantID] = e.Score
	}
	// Close-race callout.
	if len(entries) >= 3 && entries[0].Score-entries[2].Score < 5 && entries[0].Score > 0 {
		out = append(out, Event{"ticker_event", "Top 3 separated by less than 5 points", 2, now})
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
