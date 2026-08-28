package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/trade-eval/bot-fleet/telemetry"
)

// Consumer reads test commands and manages a registry of runners.
type Consumer struct {
	reader   *kafka.Reader
	cfg      Config
	logger   *slog.Logger
	producer *telemetry.Producer
	timeout  time.Duration

	mu      sync.Mutex
	runners map[string]*TestRunner
}

// NewConsumer constructs the command consumer.
func NewConsumer(cfg Config, producer *telemetry.Producer, logger *slog.Logger) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     strings.Split(cfg.KafkaBrokers, ","),
		GroupID:     cfg.ConsumerGroupID,
		Topic:       cfg.OrchEventsTopic,
		MinBytes:    1,
		MaxBytes:    10 << 20,
		StartOffset: kafka.LastOffset,
	})
	return &Consumer{
		reader:   r,
		cfg:      cfg,
		logger:   logger,
		producer: producer,
		timeout:  time.Duration(cfg.RequestTimeoutMs) * time.Millisecond,
		runners:  make(map[string]*TestRunner),
	}
}

// Run consumes until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			c.logger.Error("read", "error", err)
			continue
		}
		c.dispatch(ctx, msg.Value)
	}
}

func (c *Consumer) dispatch(ctx context.Context, raw []byte) {
	var env struct {
		Event string `json:"event"`
	}
	if json.Unmarshal(raw, &env) != nil {
		return
	}
	switch env.Event {
	case "START_TEST":
		var ev StartTestEvent
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		c.startTest(ctx, ev)
	case "STOP_TEST":
		var ev StopTestEvent
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		c.stopTest(ev.TestID, ev.Reason)
	case "CONTAINER_CRASHED":
		var ev ContainerCrashedEvent
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		c.stopByContestant(ev.ContestantID, "container_crashed")
	}
}

func (c *Consumer) startTest(ctx context.Context, ev StartTestEvent) {
	c.mu.Lock()
	if _, exists := c.runners[ev.TestID]; exists {
		c.mu.Unlock()
		return // duplicate START_TEST
	}
	runner := NewTestRunner(ev, c.producer, c.timeout, c.logger)
	c.runners[ev.TestID] = runner
	c.mu.Unlock()
	runner.Start(ctx)
}

func (c *Consumer) stopTest(testID, reason string) {
	c.mu.Lock()
	runner, ok := c.runners[testID]
	delete(c.runners, testID)
	c.mu.Unlock()
	if ok {
		runner.Stop(reason)
	}
}

func (c *Consumer) stopByContestant(contestantID, reason string) {
	c.mu.Lock()
	var toStop []*TestRunner
	for id, r := range c.runners {
		if r.contestantID == contestantID {
			toStop = append(toStop, r)
			delete(c.runners, id)
		}
	}
	c.mu.Unlock()
	for _, r := range toStop {
		r.Stop(reason)
	}
}

// ActiveTests returns the number of running tests.
func (c *Consumer) ActiveTests() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.runners)
}

// Close closes the reader and stops all runners.
func (c *Consumer) Close() error {
	c.mu.Lock()
	runners := make([]*TestRunner, 0, len(c.runners))
	for _, r := range c.runners {
		runners = append(runners, r)
	}
	c.runners = map[string]*TestRunner{}
	c.mu.Unlock()
	for _, r := range runners {
		r.Stop("shutdown")
	}
	return c.reader.Close()
}
