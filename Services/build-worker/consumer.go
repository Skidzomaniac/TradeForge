package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/segmentio/kafka-go"
)

// Consumer reads build jobs and dispatches them to a bounded worker pool.
type Consumer struct {
	reader *kafka.Reader
	logger *slog.Logger
	worker *Worker
	sem    chan struct{}
}

// NewConsumer constructs the consumer group reader.
func NewConsumer(cfg Config, logger *slog.Logger, worker *Worker) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     strings.Split(cfg.KafkaBrokers, ","),
		GroupID:     cfg.ConsumerGroupID,
		Topic:       cfg.BuildJobsTopic,
		MinBytes:    1,
		MaxBytes:    10 << 20,
		StartOffset: kafka.FirstOffset,
	})
	return &Consumer{
		reader: r,
		logger: logger,
		worker: worker,
		sem:    make(chan struct{}, cfg.MaxConcurrentBuild),
	}
}

// Run consumes until the context is cancelled. Offsets are committed only
// after ProcessBuild returns (at-least-once delivery).
func (c *Consumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			c.logger.Error("fetch message", "error", err)
			continue
		}

		var job BuildJob
		if err := json.Unmarshal(msg.Value, &job); err != nil {
			c.logger.Error("bad build job", "error", err)
			_ = c.reader.CommitMessages(ctx, msg) // poison message - skip it
			continue
		}

		c.sem <- struct{}{}
		go func(m kafka.Message, j BuildJob) {
			defer func() { <-c.sem }()
			c.worker.ProcessBuild(ctx, j)
			if err := c.reader.CommitMessages(context.Background(), m); err != nil {
				c.logger.Error("commit offset", "error", err)
			}
		}(msg, job)
	}
}

// Close closes the reader.
func (c *Consumer) Close() error { return c.reader.Close() }
