package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/segmentio/kafka-go"
)

// Consumer reads orchestrator-events and dispatches by event type.
type Consumer struct {
	reader *kafka.Reader
	sm     *TestStateMachine
	cfg    Config
	logger *slog.Logger
}

// NewConsumer constructs the event consumer.
func NewConsumer(cfg Config, sm *TestStateMachine, logger *slog.Logger) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     strings.Split(cfg.KafkaBrokers, ","),
		GroupID:     cfg.ConsumerGroupID,
		Topic:       cfg.OrchEventsTopic,
		MinBytes:    1,
		MaxBytes:    10 << 20,
		StartOffset: kafka.LastOffset,
	})
	return &Consumer{reader: r, sm: sm, cfg: cfg, logger: logger}
}

// Run consumes until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			c.logger.Error("read message", "error", err)
			continue
		}
		c.dispatch(ctx, msg.Value)
	}
}

func (c *Consumer) dispatch(ctx context.Context, raw []byte) {
	var envelope struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		c.logger.Warn("bad event json", "error", err)
		return
	}

	switch envelope.Event {
	case "CONTAINER_READY":
		var ev ContainerReadyEvent
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		if c.cfg.AutoTriggerTests {
			start := StartTestEvent{
				Event:           "START_TEST",
				TestID:          "test_auto_" + ev.SubmissionID,
				ContestantID:    ev.ContestantID,
				TargetIP:        ev.ContainerIP,
				TargetPort:      ev.ContainerPort,
				DurationSeconds: c.cfg.DefaultDurationSec,
				BotCount:        c.cfg.DefaultBotCount,
			}
			c.handleStart(ctx, start)
		} else {
			c.logger.Info("container ready, awaiting manual trigger", "submission_id", ev.SubmissionID)
		}

	case "START_TEST":
		var ev StartTestEvent
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		c.handleStart(ctx, ev)

	case "STOP_TEST":
		var ev StopTestEvent
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		if err := c.sm.StopTest(ctx, ev.TestID, ev.Reason); err != nil {
			c.logger.Warn("stop test", "test_id", ev.TestID, "error", err)
		}

	case "CONTAINER_CRASHED":
		var ev ContainerCrashedEvent
		if json.Unmarshal(raw, &ev) != nil {
			return
		}
		for testID, contestant := range c.sm.ActiveTests() {
			if contestant == ev.ContestantID {
				_ = c.sm.FailTest(ctx, testID, "container_crashed")
			}
		}

	default:
		// Unknown / informational events are ignored.
	}
}

func (c *Consumer) handleStart(ctx context.Context, ev StartTestEvent) {
	err := c.sm.StartTest(ctx, ev)
	switch {
	case err == nil:
	case errors.Is(err, ErrConcurrentModification):
		c.logger.Warn("start ignored (already handled / locked)", "test_id", ev.TestID)
	default:
		c.logger.Error("start test failed", "test_id", ev.TestID, "error", err)
		_ = c.sm.FailTest(ctx, ev.TestID, "start_failed: "+err.Error())
	}
}

// Close closes the reader.
func (c *Consumer) Close() error { return c.reader.Close() }
