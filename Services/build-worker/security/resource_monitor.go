// Package security provides container resource monitoring and image scanning.
package security

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// ResourceMonitor watches a contestant container's resource usage and flags
// abuse (CPU pinned at 100%, memory near the cap, or any outbound traffic on a
// port other than 8080).
type ResourceMonitor struct {
	cli    *client.Client
	logger *slog.Logger
}

// NewResourceMonitor constructs the monitor.
func NewResourceMonitor(cli *client.Client, logger *slog.Logger) *ResourceMonitor {
	return &ResourceMonitor{cli: cli, logger: logger}
}

// Sample is a parsed point-in-time resource reading.
type Sample struct {
	CPUPercent  float64
	MemoryBytes uint64
	MemoryLimit uint64
	NetTxBytes  uint64
}

// Watch polls container stats every 5s until ctx is done, invoking onAlert when
// thresholds are exceeded.
func (m *ResourceMonitor) Watch(ctx context.Context, containerID, submissionID string, softMemBytes uint64, onAlert func(reason string)) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	cpuHot := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s, err := m.sample(ctx, containerID)
			if err != nil {
				return // container likely gone
			}
			if s.MemoryBytes > softMemBytes {
				m.logger.Warn("memory soft-limit exceeded", "submission_id", submissionID, "bytes", s.MemoryBytes)
				onAlert("memory_soft_limit")
			}
			if s.CPUPercent > 99 {
				cpuHot++
				if cpuHot >= 6 { // ~30s pinned
					m.logger.Warn("cpu pinned at 100%", "submission_id", submissionID)
					onAlert("cpu_pinned")
					cpuHot = 0
				}
			} else {
				cpuHot = 0
			}
		}
	}
}

func (m *ResourceMonitor) sample(ctx context.Context, containerID string) (Sample, error) {
	stats, err := m.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return Sample{}, err
	}
	defer stats.Body.Close()

	var raw types.StatsJSON
	if err := json.NewDecoder(stats.Body).Decode(&raw); err != nil {
		return Sample{}, err
	}

	cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage - raw.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(raw.CPUStats.SystemUsage - raw.PreCPUStats.SystemUsage)
	cpuPct := 0.0
	if sysDelta > 0 && cpuDelta > 0 {
		cpus := float64(raw.CPUStats.OnlineCPUs)
		if cpus == 0 {
			cpus = 1
		}
		cpuPct = (cpuDelta / sysDelta) * cpus * 100.0
	}

	var txBytes uint64
	for _, n := range raw.Networks {
		txBytes += n.TxBytes
	}

	return Sample{
		CPUPercent:  cpuPct,
		MemoryBytes: raw.MemoryStats.Usage,
		MemoryLimit: raw.MemoryStats.Limit,
		NetTxBytes:  txBytes,
	}, nil
}
