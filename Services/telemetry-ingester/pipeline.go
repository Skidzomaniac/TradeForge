package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/trade-eval/telemetry-ingester/anomaly"
	"github.com/trade-eval/telemetry-ingester/metrics"
	"github.com/trade-eval/telemetry-ingester/model"
	"github.com/trade-eval/telemetry-ingester/shadowbook"
	"github.com/trade-eval/telemetry-ingester/storage"
)

// Pipeline wires reorder -> validate -> aggregate -> persist.
type Pipeline struct {
	validator *shadowbook.CorrectnessValidator
	registry  *metrics.Registry
	timescale *storage.TimescaleWriter
	redis     *storage.RedisMetricsWriter
	anomaly   *anomaly.Detector
	zscore    *anomaly.ZScoreDetector
	logger    *slog.Logger
}

// NewPipeline constructs the pipeline.
func NewPipeline(tw *storage.TimescaleWriter, rw *storage.RedisMetricsWriter, ad *anomaly.Detector, logger *slog.Logger) *Pipeline {
	return &Pipeline{
		validator: shadowbook.NewCorrectnessValidator(),
		registry:  metrics.NewRegistry(),
		timescale: tw,
		redis:     rw,
		anomaly:   ad,
		zscore:    anomaly.NewZScoreDetector(),
		logger:    logger,
	}
}

// ProcessBatch validates and records a sequence-ordered batch.
func (p *Pipeline) ProcessBatch(events []model.TelemetryEvent) {
	byContestant := make(map[string][]model.TelemetryEvent)
	for _, e := range events {
		byContestant[e.ContestantID] = append(byContestant[e.ContestantID], e)
	}
	for contestantID, evs := range byContestant {
		agg := p.registry.Get(contestantID)
		results := p.validator.ValidateBatch(evs)
		for i, e := range evs {
			agg.ProcessEvent(e, results[i])
			eventsProcessed.Inc()
			if results[i].Correct {
				eventsCorrect.Inc()
			}
			if p.anomaly != nil {
				if reps := p.anomaly.MLInspect(p.zscore, e); len(reps) > 0 {
					p.anomaly.Publish(context.Background(), reps)
				}
			}
			p.timescale.WriteSample(storage.LatencySample{
				Time:         time.Unix(0, e.SentAtNs),
				ContestantID: e.ContestantID,
				TestID:       e.TestID,
				BotID:        e.BotID,
				BotPersona:   e.BotPersona,
				LatencyUs:    e.LatencyUs,
				OrderType:    e.OrderType,
				Correct:      results[i].Correct,
				TimedOut:     e.TimedOut,
				OrderID:      e.OrderID,
				SentAtNs:     e.SentAtNs,
			})
		}
	}
}

// RunPublishers advances windows and flushes Redis metrics every second.
func (p *Pipeline) RunPublishers(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.registry.AdvanceAll()
			for _, snap := range p.registry.Snapshots() {
				_ = p.redis.PublishSnapshot(ctx, snap)
				if p.anomaly != nil {
					reports := p.anomaly.Inspect(snap)
					behavior := anomaly.Classify(snap.P50LatencyUs, snap.P99LatencyUs, snap.CorrectnessRate)
					p.redis.PublishBehavior(ctx, snap.ContestantID, behavior)
					p.anomaly.Publish(ctx, reports)
				}
			}
		}
	}
}
