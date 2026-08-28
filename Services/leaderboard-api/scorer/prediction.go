package scorer

import "sync"

// Predictor keeps a short rolling history of each contestant's score and
// projects a final score via simple linear regression.
type Predictor struct {
	mu      sync.Mutex
	history map[string][]float64
	maxLen  int
}

// NewPredictor constructs the predictor.
func NewPredictor() *Predictor {
	return &Predictor{history: make(map[string][]float64), maxLen: 60}
}

// Record appends the latest score sample for each entry.
func (p *Predictor) Record(entries []Entry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, e := range entries {
		h := append(p.history[e.ContestantID], e.Score)
		if len(h) > p.maxLen {
			h = h[len(h)-p.maxLen:]
		}
		p.history[e.ContestantID] = h
	}
}

// Prediction is the projected outcome for one contestant.
type Prediction struct {
	ContestantID   string  `json:"contestant_id"`
	CurrentScore   float64 `json:"current_score"`
	PredictedScore float64 `json:"predicted_score"`
	Trend          string  `json:"trend"` // up | down | flat
	Confidence     float64 `json:"confidence"`
	Samples        int     `json:"samples"`
}

// Predict projects the score a few cycles ahead from the recorded trend.
func (p *Predictor) Predict(contestantID string) Prediction {
	p.mu.Lock()
	h := append([]float64(nil), p.history[contestantID]...)
	p.mu.Unlock()

	n := len(h)
	pred := Prediction{ContestantID: contestantID, Samples: n}
	if n == 0 {
		return pred
	}
	pred.CurrentScore = h[n-1]
	if n < 3 {
		pred.PredictedScore = pred.CurrentScore
		pred.Trend = "flat"
		return pred
	}

	// Least-squares slope over index->score.
	var sx, sy, sxy, sxx float64
	for i, y := range h {
		x := float64(i)
		sx += x
		sy += y
		sxy += x * y
		sxx += x * x
	}
	fn := float64(n)
	denom := fn*sxx - sx*sx
	slope := 0.0
	if denom != 0 {
		slope = (fn*sxy - sx*sy) / denom
	}
	intercept := (sy - slope*sx) / fn
	// Project ~10 cycles ahead.
	projected := intercept + slope*float64(n+10)
	pred.PredictedScore = clamp(round2(projected), 0, 100)
	switch {
	case slope > 0.05:
		pred.Trend = "up"
	case slope < -0.05:
		pred.Trend = "down"
	default:
		pred.Trend = "flat"
	}
	pred.Confidence = clamp(float64(n)/float64(p.maxLen), 0, 1)
	return pred
}
