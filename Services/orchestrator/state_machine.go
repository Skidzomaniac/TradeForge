package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"
)

var ErrConcurrentModification = errors.New("concurrent modification")

type TestStateMachine struct {
	mu          sync.RWMutex
	cfg         Config
	repo        *TestRepo
	producer    *Producer
	logger      *slog.Logger
	activeTests map[string]string // testID -> contestantID
	timers      map[string]*time.Timer
}

func NewStateMachine(cfg Config, repo *TestRepo, rdb interface{}, producer *Producer, mw interface{}, logger *slog.Logger) *TestStateMachine {
	return &TestStateMachine{
		cfg:         cfg,
		repo:        repo,
		producer:    producer,
		logger:      logger,
		activeTests: make(map[string]string),
		timers:      make(map[string]*time.Timer),
	}
}

func (sm *TestStateMachine) ActiveTests() map[string]string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	res := make(map[string]string, len(sm.activeTests))
	for k, v := range sm.activeTests {
		res[k] = v
	}
	return res
}

func (sm *TestStateMachine) RegisterRecoveredTimer(testID, contestantID string, remaining time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.activeTests[testID] = contestantID
	sm.timers[testID] = time.AfterFunc(remaining, func() {
		_ = sm.StopTest(context.Background(), testID, "duration_elapsed")
	})
}

func (sm *TestStateMachine) StartTest(ctx context.Context, ev StartTestEvent) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ok, err := sm.repo.CASStatus(ctx, ev.TestID, "pending", "running", sm.cfg.InstanceID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrConcurrentModification
	}

	sm.activeTests[ev.TestID] = ev.ContestantID
	sm.timers[ev.TestID] = time.AfterFunc(time.Duration(ev.DurationSeconds)*time.Second, func() {
		_ = sm.StopTest(context.Background(), ev.TestID, "duration_elapsed")
	})

	return nil
}

func (sm *TestStateMachine) StopTest(ctx context.Context, testID, reason string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ok, err := sm.repo.CASStatus(ctx, testID, "running", "completed", sm.cfg.InstanceID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrConcurrentModification
	}

	if timer, exists := sm.timers[testID]; exists {
		timer.Stop()
		delete(sm.timers, testID)
	}

	contestantID := sm.activeTests[testID]
	delete(sm.activeTests, testID)

	sm.logger.Info("test stopped", "test_id", testID, "reason", reason)
	return sm.publishStop(ctx, testID, contestantID, reason)
}

func (sm *TestStateMachine) FailTest(ctx context.Context, testID, reason string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ok, err := sm.repo.CASStatus(ctx, testID, "running", "failed", sm.cfg.InstanceID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrConcurrentModification
	}

	_ = sm.repo.SetFailureReason(ctx, testID, reason)

	if timer, exists := sm.timers[testID]; exists {
		timer.Stop()
		delete(sm.timers, testID)
	}

	contestantID := sm.activeTests[testID]
	delete(sm.activeTests, testID)

	sm.logger.Info("test failed", "test_id", testID, "reason", reason)
	return sm.publishStop(ctx, testID, contestantID, reason)
}

func (sm *TestStateMachine) publishStop(ctx context.Context, testID, contestantID, reason string) error {
	ev := StopTestEvent{
		Event:  "STOP_TEST",
		TestID: testID,
		Reason: reason,
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return sm.producer.Produce(ctx, sm.cfg.OrchEventsTopic, []byte(testID), b)
}
