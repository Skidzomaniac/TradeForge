package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// Recovery owns orphan detection and heartbeat writing.
type Recovery struct {
	cfg    Config
	repo   *TestRepo
	redis  *redis.Client
	sm     *TestStateMachine
	logger *slog.Logger
	client *http.Client
}

// NewRecovery constructs the recovery worker.
func NewRecovery(cfg Config, repo *TestRepo, rdb *redis.Client, sm *TestStateMachine, logger *slog.Logger) *Recovery {
	return &Recovery{
		cfg:    cfg,
		repo:   repo,
		redis:  rdb,
		sm:     sm,
		logger: logger,
		client: &http.Client{Timeout: 2 * time.Second},
	}
}

// WriteHeartbeats stamps last_heartbeat_at for locally-owned tests.
func (r *Recovery) WriteHeartbeats(ctx context.Context) {
	t := time.NewTicker(time.Duration(r.cfg.HeartbeatSeconds) * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for testID := range r.sm.ActiveTests() {
				_ = r.repo.Heartbeat(ctx, testID, r.cfg.InstanceID)
				if r.redis != nil {
					r.redis.Set(ctx, "orchestrator_heartbeat:"+testID, r.cfg.InstanceID, 30*time.Second)
				}
			}
		}
	}
}

// DetectOrphans recovers tests whose owning orchestrator went silent.
func (r *Recovery) DetectOrphans(ctx context.Context) {
	t := time.NewTicker(time.Duration(r.cfg.OrphanDetectSecs) * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.recoverOnce(ctx)
		}
	}
}

func (r *Recovery) recoverOnce(ctx context.Context) {
	orphans, err := r.repo.FindOrphans(ctx, r.cfg.OrphanDetectSecs)
	if err != nil {
		r.logger.Error("find orphans", "error", err)
		return
	}
	for _, o := range orphans {
		// Only one instance recovers each test.
		if r.redis != nil {
			ok, err := r.redis.SetNX(ctx, "lock:recover:"+o.ID, r.cfg.InstanceID, 30*time.Second).Result()
			if err != nil || !ok {
				continue
			}
		}
		r.logger.Info("recovering orphaned test", "test_id", o.ID, "last_heartbeat", o.LastHeartbeat)

		if r.containerHealthy(ctx, o.ID) {
			remaining := r.remainingDuration(ctx, o.ID)
			if remaining > 0 {
				_ = r.repo.ClaimRecovery(ctx, o.ID, r.cfg.InstanceID)
				r.sm.RegisterRecoveredTimer(o.ID, o.ContestantID, remaining)
				orphansRecovered.Inc()
				r.logger.Info("orphan re-registered", "test_id", o.ID, "remaining", remaining)
			} else {
				_ = r.sm.StopTest(ctx, o.ID, "recovered_past_duration")
			}
		} else {
			_ = r.sm.FailTest(ctx, o.ID, "container_unavailable_after_recovery")
		}

		if r.redis != nil {
			r.redis.Del(ctx, "lock:recover:"+o.ID)
		}
	}
}

func (r *Recovery) containerHealthy(ctx context.Context, testID string) bool {
	if r.redis == nil {
		return false
	}
	ip, _ := r.redis.HGet(ctx, "test:"+testID, "target_ip").Result()
	port, _ := r.redis.HGet(ctx, "test:"+testID, "target_port").Result()
	if ip == "" || port == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s:%s/health", ip, port), nil)
	if err != nil {
		return false
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (r *Recovery) remainingDuration(ctx context.Context, testID string) time.Duration {
	if r.redis == nil {
		return 0
	}
	startedStr, _ := r.redis.HGet(ctx, "test:"+testID, "started_at").Result()
	started := atoiI64(startedStr)
	if started == 0 {
		return 0
	}
	elapsed := time.Since(time.Unix(0, started))
	total := time.Duration(r.cfg.DefaultDurationSec) * time.Second
	return total - elapsed
}
