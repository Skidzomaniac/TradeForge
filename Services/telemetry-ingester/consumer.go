package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/trade-eval/telemetry-ingester/model"
)

// messageSource is the subset of *kafka.Reader the consumer needs. It is an
// interface so the consumer can be tested with a fake that simulates a restart.
type messageSource interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// Consumer reads bot-telemetry and feeds the reorder buffer.
//
// Offsets are committed explicitly, never on read. FetchMessage does not commit;
// an offset is committed only after the reorder buffer has flushed the event
// into the pipeline (Timescale write enqueued, aggregator updated). On a crash
// before that point the offsets are uncommitted, so the events are redelivered
// instead of lost. Delivery is therefore at-least-once.
type Consumer struct {
	src    messageSource
	buffer *ReorderBuffer
	logger *slog.Logger
}

// NewConsumer constructs the telemetry consumer. CommitInterval is left at its
// zero value, which disables kafka-go auto-commit; offsets are committed only by
// Commit.
func NewConsumer(cfg Config, buffer *ReorderBuffer, logger *slog.Logger) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     strings.Split(cfg.KafkaBrokers, ","),
		GroupID:     cfg.ConsumerGroupID,
		Topic:       cfg.TelemetryTopic,
		MinBytes:    1024,
		MaxBytes:    10 << 20,
		MaxWait:     500 * time.Millisecond,
		StartOffset: kafka.FirstOffset,
	})
	c := &Consumer{src: r, buffer: buffer, logger: logger}
	buffer.SetCommitFn(c.Commit)
	return c
}

// Run fetches messages until ctx is done. Each message is added to the reorder
// buffer; its offset is committed later by Commit once the buffer flushes it.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		msg, err := c.src.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Error("telemetry fetch", "error", err)
			continue
		}
		var e model.TelemetryEvent
		if json.Unmarshal(msg.Value, &e) != nil {
			// Unparseable: route through the buffer so it is committed in offset
			// order and dropped exactly once rather than blocking the offset.
			c.buffer.AddUnparsed(msg)
			continue
		}
		c.buffer.Add(e, msg)
	}
}

// Commit durably records the offsets of messages whose events the pipeline has
// handled. It is invoked by the reorder buffer after each flush. A fresh context
// is used so commits still succeed during shutdown after the run context is
// canceled.
func (c *Consumer) Commit(msgs []kafka.Message) {
	if len(msgs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.src.CommitMessages(ctx, msgs...); err != nil {
		c.logger.Error("commit offsets", "error", err)
	}
}

// Close closes the reader.
func (c *Consumer) Close() error { return c.src.Close() }
