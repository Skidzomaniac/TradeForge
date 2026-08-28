package scorer

import "testing"

func TestComputeScores_Rankings(t *testing.T) {
	entries := []Entry{
		{ContestantID: "alice", TPS: 9000, P99Us: 450, CorrectnessRate: 0.999},
		{ContestantID: "carol", TPS: 11000, P99Us: 620, CorrectnessRate: 0.995},
		{ContestantID: "eve", TPS: 5000, P99Us: 1200, CorrectnessRate: 0.999},
	}
	ComputeScores(entries)
	for _, e := range entries {
		if e.Score < 0 || e.Score > 100 {
			t.Fatalf("%s out of range %v", e.ContestantID, e.Score)
		}
	}
	var lowest Entry = entries[0]
	for _, e := range entries {
		if e.Score < lowest.Score {
			lowest = e
		}
	}
	if lowest.ContestantID != "eve" {
		t.Fatalf("expected eve lowest, got %s", lowest.ContestantID)
	}
}

func TestRankEntries_DisqualifiesLowVolume(t *testing.T) {
	entries := []Entry{
		{ContestantID: "ranked", TPS: 9000, P99Us: 450, CorrectnessRate: 0.99, ValidOrders: 5000},
		{ContestantID: "tooFew", TPS: 50000, P99Us: 10, CorrectnessRate: 1.0, ValidOrders: MinValidOrders - 1},
	}
	ranked := RankEntries(entries)

	var dq, ok *Entry
	for i := range ranked {
		switch ranked[i].ContestantID {
		case "tooFew":
			dq = &ranked[i]
		case "ranked":
			ok = &ranked[i]
		}
	}
	if dq == nil || !dq.Disqualified || dq.Score != 0 || dq.Rank != 0 {
		t.Fatalf("low-volume contestant must be disqualified with score 0 and no rank: %+v", dq)
	}
	if ok == nil || ok.Disqualified || ok.Rank != 1 {
		t.Fatalf("high-volume contestant must be ranked first despite the disqualified outlier: %+v", ok)
	}
	// Disqualified entries come after ranked ones.
	if ranked[len(ranked)-1].ContestantID != "tooFew" {
		t.Fatalf("disqualified entry must be listed last")
	}
}

func TestRankEntries_AtThresholdQualifies(t *testing.T) {
	entries := []Entry{{ContestantID: "edge", TPS: 100, P99Us: 100, CorrectnessRate: 1.0, ValidOrders: MinValidOrders}}
	ranked := RankEntries(entries)
	if ranked[0].Disqualified {
		t.Fatal("exactly MinValidOrders must qualify")
	}
}

func TestComputeScores_TieBreakerData(t *testing.T) {
	entries := []Entry{
		{ContestantID: "a", TPS: 100, P99Us: 100, CorrectnessRate: 1.0},
		{ContestantID: "b", TPS: 100, P99Us: 100, CorrectnessRate: 0.99},
	}
	ComputeScores(entries)
	if entries[0].Score < entries[1].Score {
		t.Fatal("higher correctness should not score lower with equal tps/p99")
	}
}
