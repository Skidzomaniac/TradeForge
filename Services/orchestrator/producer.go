package main

import (
	"context"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// Producer is a synchronous Kafka writer.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer constructs a synchronous producer.
func NewProducer(brokers string) *Producer {
	return &Producer{writer: &kafka.Writer{
		Addr:         kafka.TCP(strings.Split(brokers, ",")...),
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireOne,
		Compression:  kafka.Snappy,
		WriteTimeout: 5 * time.Second,
		MaxAttempts:  3,
	}}
}

// Produce writes a keyed message to a topic.
func (p *Producer) Produce(ctx context.Context, topic string, key, value []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{Topic: topic, Key: key, Value: value})
}

// Close flushes and closes the writer.
func (p *Producer) Close() error { return p.writer.Close() }
