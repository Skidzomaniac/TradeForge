package telemetry

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"
)

// Producer is a non-blocking, batching Kafka producer. Bots never block on it;
// when the buffer is full, events are dropped (and counted).
type Producer struct {
	writer  *kafka.Writer
	batchCh chan Event
	dropped atomic.Int64
	sent    atomic.Int64
	done    chan struct{}
}

// NewProducer constructs the producer and starts its writer goroutine.
func NewProducer(brokers, topic string) *Producer {
	p := &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(strings.Split(brokers, ",")...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			BatchSize:    1000,
			BatchTimeout: 10 * time.Millisecond,
			Compression:  kafka.Snappy,
			Async:        true,
			RequiredAcks: kafka.RequireOne,
			WriteTimeout: 5 * time.Second,
			ReadTimeout:  5 * time.Second,
		},
		batchCh: make(chan Event, 10000),
		done:    make(chan struct{}),
	}
	go p.loop()
	return p
}

// Emit enqueues an event; returns false if dropped because the buffer is full.
func (p *Producer) Emit(e Event) bool {
	select {
	case p.batchCh <- e:
		return true
	default:
		p.dropped.Add(1)
		return false
	}
}

func (p *Producer) loop() {
	batch := make([]kafka.Message, 0, 1000)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		_ = p.writer.WriteMessages(context.Background(), batch...)
		p.sent.Add(int64(len(batch)))
		batch = batch[:0]
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-p.done:
			flush()
			return
		case e := <-p.batchCh:
			if b, err := e.ToJSON(); err == nil {
				batch = append(batch, kafka.Message{Key: []byte(e.ContestantID), Value: b})
			}
			if len(batch) >= 1000 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// DroppedCount returns the number of dropped events.
func (p *Producer) DroppedCount() int64 { return p.dropped.Load() }

// SentCount returns the number of sent events.
func (p *Producer) SentCount() int64 { return p.sent.Load() }

// Close flushes and closes the producer.
func (p *Producer) Close() error {
	close(p.done)
	return p.writer.Close()
}
