package handlers

import "testing"

func TestComputeScores_Rankings(t *testing.T) {
	entries := []leaderboardEntry{
		{ContestantID: "alice", TPS: 9000, P99Us: 450, CorrectnessRate: 0.999},
		{ContestantID: "bob", TPS: 7500, P99Us: 380, CorrectnessRate: 0.998},
		{ContestantID: "carol", TPS: 11000, P99Us: 620, CorrectnessRate: 0.995},
		{ContestantID: "eve", TPS: 5000, P99Us: 1200, CorrectnessRate: 0.999},
	}
	ComputeScores(entries)

	for _, e := range entries {
		if e.Score < 0 || e.Score > 100 {
			t.Fatalf("%s score out of range: %v", e.ContestantID, e.Score)
		}
	}

	// Eve has the lowest TPS and worst latency; she should score lowest.
	lowest := entries[0]
	for _, e := range entries {
		if e.Score < lowest.Score {
			lowest = e
		}
	}
	if lowest.ContestantID != "eve" {
		t.Fatalf("expected eve to score lowest, got %s", lowest.ContestantID)
	}
}

func TestComputeScores_SingleContestant(t *testing.T) {
	entries := []leaderboardEntry{{ContestantID: "solo", TPS: 100, P99Us: 100, CorrectnessRate: 1.0}}
	ComputeScores(entries)
	// With min==max, normalized tps=1 and norm p99=1 => 0.4*1+0.4*0+0.2*1 = 0.6 -> 60.
	if entries[0].Score != 60 {
		t.Fatalf("expected 60, got %v", entries[0].Score)
	}
}

func TestNormalize_EqualMinMax(t *testing.T) {
	if got := normalize(5, 5, 5); got != 1.0 {
		t.Fatalf("expected 1.0 when min==max, got %v", got)
	}
}
