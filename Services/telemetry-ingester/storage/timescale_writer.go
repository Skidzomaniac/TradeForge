// Package storage persists telemetry to TimescaleDB and Redis.
package storage

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LatencySample is one row destined for latency_samples.
type LatencySample struct {
	Time         time.Time
	ContestantID string
	TestID       string
	BotID        string
	BotPersona   string
	LatencyUs    int64
	OrderType    string
	Correct      bool
	TimedOut     bool
	OrderID      string
	SentAtNs     int64
}

// TimescaleWriter batches inserts using the COPY protocol.
type TimescaleWriter struct {
	pool      *pgxpool.Pool
	batchCh   chan LatencySample
	batchSize int
	dropped   atomic.Int64
	logger    *slog.Logger
	done      chan struct{} // signals the loop to flush and exit
	closed    chan struct{} // closed by the loop after the final flush completes
	closeOnce sync.Once
	insert    func([]LatencySample) // write path; defaults to bulkInsert, overridable in tests
}

// NewTimescaleWriter constructs and starts the writer.
func NewTimescaleWriter(pool *pgxpool.Pool, batchSize int, logger *slog.Logger) *TimescaleWriter {
	w := &TimescaleWriter{
		pool:      pool,
		batchCh:   make(chan LatencySample, 50000),
		batchSize: batchSize,
		logger:    logger,
		done:      make(chan struct{}),
		closed:    make(chan struct{}),
	}
	w.insert = w.bulkInsert
	go w.loop()
	return w
}

// WriteSample enqueues a sample; returns false if dropped (buffer full).
func (w *TimescaleWriter) WriteSample(s LatencySample) bool {
	select {
	case w.batchCh <- s:
		return true
	default:
		w.dropped.Add(1)
		return false
	}
}

// Dropped returns the number of dropped samples.
func (w *TimescaleWriter) Dropped() int64 { return w.dropped.Load() }

func (w *TimescaleWriter) loop() {
	batch := make([]LatencySample, 0, w.batchSize)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-w.done:
			// Drain everything still queued before the final flush so samples
			// enqueued just before shutdown are not lost, then signal closed.
			for {
				select {
				case s := <-w.batchCh:
					batch = append(batch, s)
				default:
					w.insert(batch)
					close(w.closed)
					return
				}
			}
		case s := <-w.batchCh:
			batch = append(batch, s)
			if len(batch) >= w.batchSize {
				w.insert(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				w.insert(batch)
				batch = batch[:0]
			}
		}
	}
}

func (w *TimescaleWriter) bulkInsert(samples []LatencySample) {
	if len(samples) == 0 || w.pool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows := pgx.CopyFromSlice(len(samples), func(i int) ([]any, error) {
		s := samples[i]
		return []any{s.Time, s.ContestantID, s.TestID, s.BotID, s.BotPersona,
			s.LatencyUs, s.OrderType, s.Correct, s.TimedOut, s.OrderID, s.SentAtNs}, nil
	})
	_, err := w.pool.CopyFrom(ctx, pgx.Identifier{"latency_samples"},
		[]string{"time", "contestant_id", "test_id", "bot_id", "bot_persona",
			"latency_us", "order_type", "correct", "timed_out", "order_id", "sent_at_ns"}, rows)
	if err != nil {
		w.dropped.Add(int64(len(samples)))
		w.logger.Warn("timescale copy failed", "error", err, "n", len(samples))
	}
}

// Close signals the writer to flush and blocks until the final batch has been
// written. It is safe to call more than once. Callers must close the writer
// before closing the connection pool, otherwise the final flush would run
// against a closed pool.
func (w *TimescaleWriter) Close() {
	w.closeOnce.Do(func() { close(w.done) })
	<-w.closed
}
