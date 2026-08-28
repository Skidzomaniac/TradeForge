// Package kafka provides a synchronous Kafka producer for submission-api.
package kafka

import (
	"context"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// Producer is a thin synchronous wrapper around kafka-go's Writer.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer builds a synchronous producer (RequireOne acks, snappy).
func NewProducer(brokers string) *Producer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(strings.Split(brokers, ",")...),
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireOne,
		Compression:  kafka.Snappy,
		WriteTimeout: 5 * time.Second,
		MaxAttempts:  3,
		Async:        false,
	}
	return &Producer{writer: w}
}

// Produce synchronously writes a single keyed message to a topic.
func (p *Producer) Produce(ctx context.Context, topic string, key, value []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   key,
		Value: value,
	})
}

// Close flushes and closes the underlying writer.
func (p *Producer) Close() error {
	return p.writer.Close()
}
