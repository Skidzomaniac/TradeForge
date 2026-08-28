package client

import (
	"sync"
	"time"
)

// State is a circuit-breaker state.
type State int

const (
	// Closed allows requests through.
	Closed State = iota
	// Open rejects requests fast.
	Open
	// HalfOpen probes recovery.
	HalfOpen
)

// CircuitBreaker guards one target endpoint.
type CircuitBreaker struct {
	mu             sync.Mutex
	state          State
	windowStart    time.Time
	total          int
	failures       int
	openedAt       time.Time
	halfOpenOK     int
	minTraffic     int
	failureRate    float64
	openCooldown   time.Duration
	halfOpenNeeded int
}

// NewCircuitBreaker constructs a breaker with sensible defaults.
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		state:          Closed,
		windowStart:    time.Now(),
		minTraffic:     20,
		failureRate:    0.5,
		openCooldown:   5 * time.Second,
		halfOpenNeeded: 3,
	}
}

// Allow reports whether a request may proceed.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case Open:
		if time.Since(cb.openedAt) >= cb.openCooldown {
			cb.state = HalfOpen
			cb.halfOpenOK = 0
			return true
		}
		return false
	default:
		return true
	}
}

// Record updates the breaker with a request outcome.
func (cb *CircuitBreaker) Record(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Reset the rolling window every 10s.
	if time.Since(cb.windowStart) > 10*time.Second {
		cb.windowStart = time.Now()
		cb.total = 0
		cb.failures = 0
	}

	switch cb.state {
	case HalfOpen:
		if success {
			cb.halfOpenOK++
			if cb.halfOpenOK >= cb.halfOpenNeeded {
				cb.state = Closed
				cb.total, cb.failures = 0, 0
			}
		} else {
			cb.trip()
		}
	default:
		cb.total++
		if !success {
			cb.failures++
		}
		if cb.total >= cb.minTraffic && float64(cb.failures)/float64(cb.total) > cb.failureRate {
			cb.trip()
		}
	}
}

func (cb *CircuitBreaker) trip() {
	cb.state = Open
	cb.openedAt = time.Now()
}

// State returns the current state (for metrics).
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
