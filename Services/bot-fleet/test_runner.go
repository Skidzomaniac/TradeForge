package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/trade-eval/bot-fleet/bots"
	"github.com/trade-eval/bot-fleet/client"
	"github.com/trade-eval/bot-fleet/telemetry"
)

// personaRatios defines the relative weight of each persona across a test.
var personaRatios = map[string]float64{
	"market_maker":     0.40,
	"aggressive_taker": 0.30,
	"spammer":          0.20,
	"whale":            0.10,
}

// TestRunner manages one running test's bot population.
type TestRunner struct {
	testID       string
	contestantID string
	targetURL    string
	botCount     int
	personas     []string
	duration     time.Duration
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	producer     *telemetry.Producer
	logger       *slog.Logger
	seq          atomic.Int64
	transport    *client.OrderBookClient
	breaker      *client.CircuitBreaker
}

// NewTestRunner constructs a runner.
func NewTestRunner(ev StartTestEvent, producer *telemetry.Producer, timeout time.Duration, logger *slog.Logger) *TestRunner {
	personas := ev.BotPersonas
	if len(personas) == 0 {
		personas = []string{"market_maker", "aggressive_taker", "spammer", "whale"}
	}
	url := fmt.Sprintf("http://%s:%d", ev.TargetIP, ev.TargetPort)
	return &TestRunner{
		testID:       ev.TestID,
		contestantID: ev.ContestantID,
		targetURL:    url,
		botCount:     ev.BotCount,
		personas:     personas,
		duration:     time.Duration(ev.DurationSeconds) * time.Second,
		producer:     producer,
		logger:       logger,
		transport:    client.New(url, bots.SharedTransport(), timeout),
		breaker:      client.NewCircuitBreaker(),
	}
}

// resetOrderBook calls POST /reset on the contestant container to clear any
// accumulated state from previous tests so the shadow book and the C++ book
// both start from the same empty state.
func (r *TestRunner) resetOrderBook(ctx context.Context) {
	resetCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := r.transport.Reset(resetCtx); err != nil {
		r.logger.Warn("orderbook reset failed (non-fatal)", "test_id", r.testID, "error", err)
	} else {
		r.logger.Info("orderbook reset OK", "test_id", r.testID)
	}
}

// Start spawns bots and schedules the stop timer.
func (r *TestRunner) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel

	// Reset the contestant's orderbook so both the C++ server and the shadow
	// book start from the same clean state.
	r.resetOrderBook(ctx)

	assignment := r.assignPersonas()
	d := bots.Deps{
		Client:       r.transport,
		Breaker:      r.breaker,
		Producer:     r.producer,
		ContestantID: r.contestantID,
		TestID:       r.testID,
		Seq:          &r.seq,
	}

	// Run the serialized correctness phase to completion before any concurrent
	// load bot starts. This guarantees every correctness event carries a lower
	// sequence number than every load event, so the ingester's reference book
	// replays the serialized stream first and judges it exactly. Bounded so a
	// hung contestant cannot stall the whole test.
	corrCtx, corrCancel := context.WithTimeout(ctx, 30*time.Second)
	bots.RunCorrectnessPhase(corrCtx, fmt.Sprintf("corr_%s", r.testID), d)
	corrCancel()
	if ctx.Err() != nil {
		return
	}

	for i, persona := range assignment {
		r.wg.Add(1)
		botID := fmt.Sprintf("bot_%s_%d", r.testID, i)
		go func(id, p string) {
			defer r.wg.Done()
			bots.RunBot(ctx, id, p, d)
		}(botID, persona)
	}

	if r.duration > 0 {
		time.AfterFunc(r.duration, func() { r.Stop("duration_elapsed") })
	}
	r.logger.Info("test runner started", "test_id", r.testID, "bots", len(assignment))
}

// assignPersonas distributes personas across botCount by configured ratios.
func (r *TestRunner) assignPersonas() []string {
	out := make([]string, 0, r.botCount)
	for _, p := range r.personas {
		ratio, ok := personaRatios[p]
		if !ok {
			ratio = 1.0 / float64(len(r.personas))
		}
		n := int(float64(r.botCount) * ratio)
		for i := 0; i < n; i++ {
			out = append(out, p)
		}
	}
	// Fill any rounding remainder with the first persona.
	for len(out) < r.botCount {
		out = append(out, r.personas[0])
	}
	return out[:r.botCount]
}

// Stop cancels all bots and waits (bounded) for drain.
func (r *TestRunner) Stop(reason string) {
	if r.cancel == nil {
		return
	}
	r.cancel()
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		r.logger.Warn("bot drain timed out", "test_id", r.testID)
	}
	r.logger.Info("test runner stopped", "test_id", r.testID, "reason", reason)
}
