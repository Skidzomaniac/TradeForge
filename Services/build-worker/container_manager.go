package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ContainerInfo tracks one running sandbox.
type ContainerInfo struct {
	ContainerID  string
	ImageName    string
	IP           string
	Port         int
	ContestantID string
	CreatedAt    time.Time
}

// ContainerManager owns the lifecycle of all contestant sandboxes.
type ContainerManager struct {
	cli    *client.Client
	pool   *pgxpool.Pool
	redis  *redis.Client
	logger *slog.Logger

	mu     sync.RWMutex
	active map[string]ContainerInfo
}

// NewContainerManager constructs the manager.
func NewContainerManager(cli *client.Client, pool *pgxpool.Pool, rdb *redis.Client, logger *slog.Logger) *ContainerManager {
	return &ContainerManager{
		cli:    cli,
		pool:   pool,
		redis:  rdb,
		logger: logger,
		active: make(map[string]ContainerInfo),
	}
}

// RecoverActive re-populates the active map from the DB on startup so that
// the cleanup job does not treat previously-launched containers as orphans.
func (m *ContainerManager) RecoverActive(ctx context.Context) {
	if m.pool == nil {
		return
	}
	rows, err := m.pool.Query(ctx,
		`SELECT id, contestant_id, container_id, container_ip, container_port
		   FROM submissions WHERE status = 'ready' AND container_id IS NOT NULL`)
	if err != nil {
		m.logger.Warn("RecoverActive: query failed", "error", err)
		return
	}
	defer rows.Close()
	recovered := 0
	for rows.Next() {
		var subID, contestantID, containerID, ip string
		var port int
		if err := rows.Scan(&subID, &contestantID, &containerID, &ip, &port); err != nil {
			continue
		}
		// Verify the container is still running in Docker.
		inspect, err := m.cli.ContainerInspect(ctx, containerID)
		if err != nil || inspect.State == nil || !inspect.State.Running {
			continue
		}
		m.mu.Lock()
		m.active[subID] = ContainerInfo{
			ContainerID:  containerID,
			IP:           ip,
			Port:         port,
			ContestantID: contestantID,
			CreatedAt:    time.Now(),
		}
		m.mu.Unlock()
		recovered++
	}
	if recovered > 0 {
		m.logger.Info("RecoverActive: re-registered containers", "count", recovered)
	}
}

// Track records a running container.
func (m *ContainerManager) Track(submissionID string, info ContainerInfo) {
	m.mu.Lock()
	m.active[submissionID] = info
	m.mu.Unlock()
}

// Get returns the tracked container info.
func (m *ContainerManager) Get(submissionID string) (ContainerInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.active[submissionID]
	return info, ok
}

// StopContainer gracefully stops and removes a sandbox and its image.
func (m *ContainerManager) StopContainer(ctx context.Context, submissionID string) error {
	info, ok := m.Get(submissionID)
	if !ok {
		return nil
	}
	timeout := 10
	_ = m.cli.ContainerStop(ctx, info.ContainerID, container.StopOptions{Timeout: &timeout})
	_ = m.cli.ContainerRemove(ctx, info.ContainerID, container.RemoveOptions{Force: true})
	if info.ImageName != "" {
		_, _ = m.cli.ImageRemove(ctx, info.ImageName, types.ImageRemoveOptions{Force: true, PruneChildren: true})
	}

	m.mu.Lock()
	delete(m.active, submissionID)
	m.mu.Unlock()

	if m.redis != nil {
		m.redis.Del(ctx, "container:"+submissionID)
	}
	if m.pool != nil {
		_, _ = m.pool.Exec(ctx, `UPDATE submissions SET status='stopped' WHERE id=$1`, submissionID)
	}
	m.logger.Info("stopped container", "submission_id", submissionID)
	return nil
}

// StopAll stops every tracked container; blocks until done.
func (m *ContainerManager) StopAll(ctx context.Context) {
	m.mu.RLock()
	ids := make([]string, 0, len(m.active))
	for id := range m.active {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		_ = m.StopContainer(ctx, id)
	}
}

// MonitorHealth probes tracked containers every 30s and marks dead ones.
func (m *ContainerManager) MonitorHealth(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.checkAll(ctx)
		}
	}
}

func (m *ContainerManager) checkAll(ctx context.Context) {
	m.mu.RLock()
	snapshot := make(map[string]ContainerInfo, len(m.active))
	for id, info := range m.active {
		snapshot[id] = info
	}
	m.mu.RUnlock()

	for id, info := range snapshot {
		inspect, err := m.cli.ContainerInspect(ctx, info.ContainerID)
		if err != nil || inspect.State == nil || !inspect.State.Running {
			m.logger.Warn("container unhealthy", "submission_id", id)
			if m.redis != nil {
				m.redis.HSet(ctx, "container:"+id, "status", "dead")
			}
			if m.pool != nil {
				_, _ = m.pool.Exec(ctx, `UPDATE submissions SET status='failed', error_log='container died' WHERE id=$1`, id)
			}
			m.mu.Lock()
			delete(m.active, id)
			m.mu.Unlock()
		}
	}
}
