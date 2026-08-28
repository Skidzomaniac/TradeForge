package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/trade-eval/telemetry-ingester/model"
)

// fakeSource is an in-memory messageSource. It delivers a fixed list of messages
// in order, then blocks until the context is canceled, and records every offset
// that is committed so a test can assert the commit watermark.
type fakeSource struct {
	mu        sync.Mutex
	msgs      []kafka.Message
	idx       int
	committed []int64
	delivered chan struct{} // closed once every message has been fetched
	once      sync.Once
}

func newFakeSource(msgs []kafka.Message) *fakeSource {
	return &fakeSource{msgs: msgs, delivered: make(chan struct{})}
}

func (f *fakeSource) FetchMessage(ctx context.Context) (kafka.Message, error) {
	f.mu.Lock()
	if f.idx < len(f.msgs) {
		m := f.msgs[f.idx]
		f.idx++
		last := f.idx == len(f.msgs)
		f.mu.Unlock()
		if last {
			f.once.Do(func() { close(f.delivered) })
		}
		return m, nil
	}
	f.mu.Unlock()
	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func (f *fakeSource) CommitMessages(_ context.Context, msgs ...kafka.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, m := range msgs {
		f.committed = append(f.committed, m.Offset)
	}
	return nil
}

func (f *fakeSource) Close() error { return nil }

func (f *fakeSource) maxCommitted() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	max := int64(-1)
	for _, o := range f.committed {
		if o > max {
			max = o
		}
	}
	return max
}

// makeMsg builds a kafka message whose value is a valid telemetry event. The
// offset is mirrored into SequenceNumber so a test can match committed offsets
// against persisted events.
func makeMsg(offset int64) kafka.Message {
	e := model.TelemetryEvent{ContestantID: "c1", TestID: "t1", OrderID: "o", SequenceNumber: offset}
	b, _ := json.Marshal(e)
	return kafka.Message{Offset: offset, Value: b}
}

func newTestConsumer(src messageSource, buffer *ReorderBuffer) *Consumer {
	c := &Consumer{src: src, buffer: buffer, logger: slog.Default()}
	buffer.SetCommitFn(c.Commit)
	return c
}

// TestConsumerCommitsOnlyAfterFlush asserts the happy path: messages are
// committed after the buffer flushes them into the pipeline.
func TestConsumerCommitsOnlyAfterFlush(t *testing.T) {
	src := newFakeSource([]kafka.Message{makeMsg(0), makeMsg(1), makeMsg(2)})

	var mu sync.Mutex
	persisted := map[int64]bool{}
	buffer := NewReorderBuffer(time.Hour, func(evs []model.TelemetryEvent) {
		mu.Lock()
		for _, e := range evs {
			persisted[e.SequenceNumber] = true
		}
		mu.Unlock()
	})
	c := newTestConsumer(src, buffer)

	ctx, cancel := context.WithCancel(context.Background())
	go c.Run(ctx)

	<-src.delivered // all three fetched and in the buffer (or about to be)
	waitFor(t, func() bool { return buffer.Len() == 3 })

	// Nothing committed before a flush.
	if got := src.maxCommitted(); got != -1 {
		t.Fatalf("committed before flush: max offset %d", got)
	}

	buffer.flush() // pipeline handles the batch, then offsets commit
	cancel()

	if got := src.maxCommitted(); got != 2 {
		t.Fatalf("want committed up to offset 2, got %d", got)
	}
	mu.Lock()
	defer mu.Unlock()
	for off := int64(0); off <= 2; off++ {
		if !persisted[off] {
			t.Fatalf("offset %d committed but not persisted", off)
		}
	}
}

// TestConsumerCrashLeavesUnpersistedUncommitted is the durability regression for
// item 4: a crash after fetching but before the pipeline handles the batch must
// leave those offsets uncommitted, so they are redelivered on restart rather
// than silently lost.
func TestConsumerCrashLeavesUnpersistedUncommitted(t *testing.T) {
	src := newFakeSource([]kafka.Message{makeMsg(0), makeMsg(1), makeMsg(2)})

	var mu sync.Mutex
	persisted := map[int64]bool{}
	buffer := NewReorderBuffer(time.Hour, func(evs []model.TelemetryEvent) {
		mu.Lock()
		for _, e := range evs {
			persisted[e.SequenceNumber] = true
		}
		mu.Unlock()
	})
	c := newTestConsumer(src, buffer)

	ctx, cancel := context.WithCancel(context.Background())
	go c.Run(ctx)

	<-src.delivered
	waitFor(t, func() bool { return buffer.Len() == 3 })

	// Crash: cancel before any flush. The buffer's in-memory contents are lost.
	cancel()

	if got := src.maxCommitted(); got != -1 {
		t.Fatalf("offsets committed without persistence on crash: max %d", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(persisted) != 0 {
		t.Fatalf("nothing should have persisted, got %d", len(persisted))
	}
	// On restart the reader resumes from the uncommitted offset 0, so no event is
	// lost. This is the no committed-but-unpersisted gap guarantee.
}

// TestConsumerUnparsedMessageCommitted asserts a malformed message does not wedge
// the offset: it is committed once and dropped, not redelivered forever.
func TestConsumerUnparsedMessageCommitted(t *testing.T) {
	bad := kafka.Message{Offset: 0, Value: []byte("{not json")}
	good := makeMsg(1)
	src := newFakeSource([]kafka.Message{bad, good})

	var persistedCount int
	buffer := NewReorderBuffer(time.Hour, func(evs []model.TelemetryEvent) { persistedCount += len(evs) })
	c := newTestConsumer(src, buffer)

	ctx, cancel := context.WithCancel(context.Background())
	go c.Run(ctx)
	<-src.delivered
	waitFor(t, func() bool { return buffer.Len() == 2 })
	buffer.flush()
	cancel()

	if got := src.maxCommitted(); got != 1 {
		t.Fatalf("want committed up to offset 1, got %d", got)
	}
	if persistedCount != 1 {
		t.Fatalf("only the valid event should persist, got %d", persistedCount)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}
