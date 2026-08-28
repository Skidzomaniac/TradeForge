package main

import (
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/trade-eval/telemetry-ingester/model"
)

// TestReorderBufferSortsBySequence asserts a flush emits events in send-time
// sequence order regardless of arrival order.
func TestReorderBufferSortsBySequence(t *testing.T) {
	var got []int64
	rb := NewReorderBuffer(time.Hour, func(evs []model.TelemetryEvent) {
		for _, e := range evs {
			got = append(got, e.SequenceNumber)
		}
	})
	rb.Add(model.TelemetryEvent{SequenceNumber: 3}, kafka.Message{})
	rb.Add(model.TelemetryEvent{SequenceNumber: 1}, kafka.Message{})
	rb.Add(model.TelemetryEvent{SequenceNumber: 2}, kafka.Message{})
	rb.flush()

	want := []int64{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("want %v got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("out of order: want %v got %v", want, got)
		}
	}
}

// TestReorderBufferCommitsAfterFlush asserts commitFn runs after flushFn, and is
// passed exactly the messages in the flushed batch.
func TestReorderBufferCommitsAfterFlush(t *testing.T) {
	order := []string{}
	var committed []int64
	rb := NewReorderBuffer(time.Hour, func(evs []model.TelemetryEvent) {
		order = append(order, "flush")
	})
	rb.SetCommitFn(func(msgs []kafka.Message) {
		order = append(order, "commit")
		for _, m := range msgs {
			committed = append(committed, m.Offset)
		}
	})
	rb.Add(model.TelemetryEvent{SequenceNumber: 1}, kafka.Message{Offset: 10})
	rb.Add(model.TelemetryEvent{SequenceNumber: 2}, kafka.Message{Offset: 11})
	rb.flush()

	if len(order) != 2 || order[0] != "flush" || order[1] != "commit" {
		t.Fatalf("commit must follow flush, got %v", order)
	}
	if len(committed) != 2 || committed[0] != 10 || committed[1] != 11 {
		t.Fatalf("want offsets [10 11], got %v", committed)
	}
}

// TestReorderBufferEmptyFlushNoCommit asserts an empty buffer neither flushes nor
// commits, so it cannot advance the offset past unhandled data.
func TestReorderBufferEmptyFlushNoCommit(t *testing.T) {
	calls := 0
	rb := NewReorderBuffer(time.Hour, func([]model.TelemetryEvent) { calls++ })
	rb.SetCommitFn(func([]kafka.Message) { calls++ })
	rb.flush()
	if calls != 0 {
		t.Fatalf("empty flush triggered callbacks: %d", calls)
	}
}
