package main

import (
	"context"
	"sort"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// RunCleanup periodically removes orphaned containers and prunes old images.
func (m *ContainerManager) RunCleanup(ctx context.Context) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.cleanupOrphans(ctx)
			m.pruneImages(ctx)
		}
	}
}

func (m *ContainerManager) cleanupOrphans(ctx context.Context) {
	f := filters.NewArgs()
	f.Add("label", "trade-eval=contestant")
	list, err := m.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		m.logger.Error("cleanup list", "error", err)
		return
	}

	// Build set of container IDs that are tracked in memory.
	m.mu.RLock()
	tracked := make(map[string]bool, len(m.active))
	for _, info := range m.active {
		tracked[info.ContainerID] = true
	}
	m.mu.RUnlock()

	// Also load container IDs tracked in the DB (status='ready') so we never
	// kill a container that a previous build-worker instance launched and that
	// the DB still considers active - this prevents accidental teardown on restart.
	dbTracked := make(map[string]bool)
	if m.pool != nil {
		rows, qErr := m.pool.Query(ctx,
			`SELECT container_id FROM submissions WHERE status='ready' AND container_id IS NOT NULL`)
		if qErr == nil {
			for rows.Next() {
				var cid string
				if rows.Scan(&cid) == nil && cid != "" {
					dbTracked[cid] = true
				}
			}
			rows.Close()
		}
	}

	cleaned := 0
	for _, c := range list {
		if tracked[c.ID] || dbTracked[c.ID] {
			continue // in-memory or DB says it is legitimate - leave it alone
		}
		timeout := 10
		_ = m.cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})
		if err := m.cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err == nil {
			cleaned++
		}
	}
	if cleaned > 0 {
		m.logger.Info("cleaned orphaned containers", "count", cleaned)
	}
}

func (m *ContainerManager) pruneImages(ctx context.Context) {
	f := filters.NewArgs()
	f.Add("reference", "trade-eval-contestant:*")
	images, err := m.cli.ImageList(ctx, types.ImageListOptions{Filters: f})
	if err != nil {
		return
	}

	var total int64
	for _, img := range images {
		total += img.Size
	}
	const tenGB = int64(10) * 1024 * 1024 * 1024
	const fiveGB = int64(5) * 1024 * 1024 * 1024
	if total <= tenGB {
		return
	}

	sort.Slice(images, func(i, j int) bool { return images[i].Created < images[j].Created })
	for _, img := range images {
		if total <= fiveGB {
			break
		}
		if _, err := m.cli.ImageRemove(ctx, img.ID, types.ImageRemoveOptions{Force: true, PruneChildren: true}); err == nil {
			total -= img.Size
		}
	}
	m.logger.Info("pruned contestant images", "remaining_bytes", total)
}
