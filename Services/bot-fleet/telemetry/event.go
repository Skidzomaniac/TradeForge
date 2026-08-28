// Package telemetry defines the order-event schema and a high-throughput
// Kafka producer for the bot fleet.
package telemetry

import (
	"bytes"
	"encoding/json"
	"sync"
)

// Fill mirrors proto Fill.
type Fill struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Status   string  `json:"status"` // FILLED/PARTIAL/REJECTED/PENDING/CANCELLED
}

// Event is the per-order telemetry record (matches proto OrderEvent as JSON).
type Event struct {
	ContestantID   string  `json:"contestant_id"`
	TestID         string  `json:"test_id"`
	BotID          string  `json:"bot_id"`
	BotPersona     string  `json:"bot_persona"`
	Phase          string  `json:"phase"`
	OrderID        string  `json:"order_id"`
	SentAtNs       int64   `json:"sent_at_ns"`
	AckedAtNs      int64   `json:"acked_at_ns"`
	LatencyUs      int64   `json:"latency_us"`
	OrderType      string  `json:"order_type"`
	Price          float64 `json:"price"`
	Quantity       float64 `json:"quantity"`
	ExpectedFill   Fill    `json:"expected_fill"`
	ActualFill     Fill    `json:"actual_fill"`
	Correct        bool    `json:"correct"`
	TimedOut       bool    `json:"timed_out"`
	BotError       bool    `json:"bot_error"`
	SequenceNumber int64   `json:"sequence_number"`
}

var encoderPool = sync.Pool{
	New: func() any { return bytes.NewBuffer(make([]byte, 0, 512)) },
}

// ToJSON serializes the event using a pooled buffer to limit allocations at
// high event rates.
func (e *Event) ToJSON() ([]byte, error) {
	buf := encoderPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer encoderPool.Put(buf)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, nil
}
