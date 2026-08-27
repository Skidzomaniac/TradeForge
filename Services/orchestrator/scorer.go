package main

// computeCompositeScore normalizes the contestant's metrics against the field
// and returns a 0-100 composite score rounded to 2 decimals.
//
//	score = 0.40*norm(tps) + 0.40*(1-norm(p99)) + 0.20*correctness
func computeCompositeScore(metrics LatencyWindow, all []LatencyWindow) float64 {
	if len(all) == 0 {
		all = []LatencyWindow{metrics}
	}
	minTPS, maxTPS := all[0].TPS, all[0].TPS
	minP99, maxP99 := float64(all[0].P99Us), float64(all[0].P99Us)
	for _, m := range all {
		p99 := float64(m.P99Us)
		if m.TPS < minTPS {
			minTPS = m.TPS
		}
		if m.TPS > maxTPS {
			maxTPS = m.TPS
		}
		if p99 < minP99 {
			minP99 = p99
		}
		if p99 > maxP99 {
			maxP99 = p99
		}
	}

	normTPS := norm(metrics.TPS, minTPS, maxTPS)
	normInvP99 := 1.0 - norm(float64(metrics.P99Us), minP99, maxP99)
	score := 0.40*normTPS + 0.40*normInvP99 + 0.20*metrics.CorrectnessRate
	score = clamp(score*100, 0, 100)
	return round2(score)
}

func norm(v, min, max float64) float64 {
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
