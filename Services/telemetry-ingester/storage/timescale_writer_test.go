package storage

import (
	"log/slog"
	"sync"
	"testing"
	"time"
)

// newTestWriter builds a writer with the DB write path replaced by a recorder so
// the flush-on-close behavior can be tested without TimescaleDB.
func newTestWriter(batchSize int) (*TimescaleWriter, func() int) {
	var mu sync.Mutex
	var written int
	w := &TimescaleWriter{
		batchCh:   make(chan LatencySample, 1000),
		batchSize: batchSize,
		logger:    slog.Default(),
		done:      make(chan struct{}),
		closed:    make(chan struct{}),
	}
	w.insert = func(b []LatencySample) {
		mu.Lock()
		written += len(b)
		mu.Unlock()
	}
	go w.loop()
	return w, func() int { mu.Lock(); defer mu.Unlock(); return written }
}

// TestCloseFlushesFinalBatch is the regression for item 13: Close must block
// until the queued samples have been flushed, so the final batch is not lost
// when main closes the pool right after.
func TestCloseFlushesFinalBatch(t *testing.T) {
	w, count := newTestWriter(10000) // big batch so size never triggers a flush
	for i := 0; i < 5; i++ {
		if !w.WriteSample(LatencySample{ContestantID: "c1"}) {
			t.Fatal("sample dropped unexpectedly")
		}
	}
	w.Close() // must drain and flush the 5 queued samples before returning
	if got := count(); got != 5 {
		t.Fatalf("want 5 samples flushed on close, got %d", got)
	}
}

// TestCloseIsIdempotent asserts a second Close does not panic or block.
func TestCloseIsIdempotent(t *testing.T) {
	w, _ := newTestWriter(10)
	w.Close()
	done := make(chan struct{})
	go func() { w.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second Close blocked")
	}
}

// TestWriteAfterCloseDoesNotPanic guards the shutdown race where a late
// WriteSample arrives after Close; it must not panic.
func TestWriteAfterCloseDoesNotPanic(t *testing.T) {
	w, _ := newTestWriter(10)
	w.Close()
	_ = w.WriteSample(LatencySample{ContestantID: "c1"}) // buffered or dropped, never panics
}
