package main

import (
	"sort"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/trade-eval/telemetry-ingester/model"
)

// pendingEvent pairs a telemetry event with the Kafka message it arrived on so
// the offset can be committed only after the event has been handled. parsed is
// false for a message that failed to decode: it carries no event but is still
// committed in order so it is not redelivered forever.
type pendingEvent struct {
	event  model.TelemetryEvent
	msg    kafka.Message
	parsed bool
}

// ReorderBuffer holds events briefly and emits them sorted by sequence number,
// correcting for out-of-order delivery across distributed bots.
//
// Determinism vs freshness: events are held for hold and flushed in sequence
// order, so a late arrival within the window is still placed correctly. The
// trade is up to hold of added latency before an event is scored. The serialized
// correctness phase does not rely on this window: its events carry strictly
// increasing send-time sequence numbers and are emitted before any load event,
// so they sort correctly regardless of the window size.
type ReorderBuffer struct {
	mu       sync.Mutex
	pending  []pendingEvent
	flushFn  func([]model.TelemetryEvent)
	commitFn func([]kafka.Message) // invoked after flushFn returns; may be nil
	hold     time.Duration
}

// NewReorderBuffer constructs a buffer that flushes sorted batches every hold.
func NewReorderBuffer(hold time.Duration, flushFn func([]model.TelemetryEvent)) *ReorderBuffer {
	return &ReorderBuffer{flushFn: flushFn, hold: hold}
}

// SetCommitFn registers the offset-commit callback run after each flush has been
// handled by flushFn. It is set separately from the constructor so the buffer
// can be unit tested without Kafka.
func (rb *ReorderBuffer) SetCommitFn(fn func([]kafka.Message)) { rb.commitFn = fn }

// Add enqueues a decoded event and the message it came from.
func (rb *ReorderBuffer) Add(e model.TelemetryEvent, msg kafka.Message) {
	rb.mu.Lock()
	rb.pending = append(rb.pending, pendingEvent{event: e, msg: msg, parsed: true})
	rb.mu.Unlock()
}

// AddUnparsed enqueues a message that failed to decode. It produces no event but
// is committed in order on the next flush so it is dropped exactly once instead
// of being redelivered forever.
func (rb *ReorderBuffer) AddUnparsed(msg kafka.Message) {
	rb.mu.Lock()
	rb.pending = append(rb.pending, pendingEvent{msg: msg, parsed: false})
	rb.mu.Unlock()
}

// Len reports the number of buffered entries. Used in tests.
func (rb *ReorderBuffer) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return len(rb.pending)
}

// Run flushes sorted batches until stop is closed, then flushes once more.
func (rb *ReorderBuffer) Run(stop <-chan struct{}) {
	t := time.NewTicker(rb.hold)
	defer t.Stop()
	for {
		select {
		case <-stop:
			rb.flush()
			return
		case <-t.C:
			rb.flush()
		}
	}
}

func (rb *ReorderBuffer) flush() {
	rb.mu.Lock()
	if len(rb.pending) == 0 {
		rb.mu.Unlock()
		return
	}
	batch := rb.pending
	rb.pending = nil
	rb.mu.Unlock()

	sort.SliceStable(batch, func(i, j int) bool {
		return batch[i].event.SequenceNumber < batch[j].event.SequenceNumber
	})

	events := make([]model.TelemetryEvent, 0, len(batch))
	msgs := make([]kafka.Message, 0, len(batch))
	for _, pe := range batch {
		if pe.parsed {
			events = append(events, pe.event)
		}
		msgs = append(msgs, pe.msg)
	}

	// Persist first, commit second. A crash between the two redelivers the batch
	// (at-least-once) rather than dropping it. Offsets are committed only for
	// messages whose events have been handed to the pipeline.
	if len(events) > 0 {
		rb.flushFn(events)
	}
	if rb.commitFn != nil {
		rb.commitFn(msgs)
	}
}
