// Package kafka holds consumer-group management and lag monitoring helpers.
package kafka

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"
)

// LagMonitor periodically reports total consumer-group lag for a topic.
type LagMonitor struct {
	brokers   []string
	topic     string
	groupID   string
	maxLag    int64
	logger    *slog.Logger
	lastLag   atomic.Int64
	growing   atomic.Bool
	intervalS int
}

// NewLagMonitor constructs the monitor.
func NewLagMonitor(brokers, topic, groupID string, maxLag int64, logger *slog.Logger) *LagMonitor {
	return &LagMonitor{
		brokers:   strings.Split(brokers, ","),
		topic:     topic,
		groupID:   groupID,
		maxLag:    maxLag,
		logger:    logger,
		intervalS: 30,
	}
}

// Run polls lag every 30s until ctx is done.
func (m *LagMonitor) Run(ctx context.Context) {
	t := time.NewTicker(time.Duration(m.intervalS) * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.check(ctx)
		}
	}
}

// CurrentLag returns the last measured total lag.
func (m *LagMonitor) CurrentLag() int64 { return m.lastLag.Load() }

func (m *LagMonitor) check(ctx context.Context) {
	conn, err := kafka.DialContext(ctx, "tcp", m.brokers[0])
	if err != nil {
		m.logger.Warn("lag monitor dial failed", "error", err)
		return
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions(m.topic)
	if err != nil {
		return
	}

	var total int64
	for _, p := range partitions {
		// End offset for the partition.
		leader, err := kafka.DialLeader(ctx, "tcp", m.brokers[0], m.topic, p.ID)
		if err != nil {
			continue
		}
		last, err := leader.ReadLastOffset()
		leader.Close()
		if err != nil {
			continue
		}
		committed := readCommitted(ctx, m.brokers[0], m.groupID, m.topic, p.ID)
		if last > committed {
			total += last - committed
		}
	}

	prev := m.lastLag.Swap(total)
	m.growing.Store(total > int64(float64(prev)*1.2))

	switch {
	case total > m.maxLag:
		m.logger.Error("KAFKA LAG HIGH", "lag", total, "topic", m.topic)
	case m.growing.Load() && prev > 0:
		m.logger.Warn("kafka lag growing - consider more consumers", "lag", total)
	default:
		m.logger.Debug("kafka lag", "lag", total)
	}
}

// readCommitted fetches the group's committed offset for a partition.
func readCommitted(ctx context.Context, broker, group, topic string, partition int) int64 {
	client := &kafka.Client{Addr: kafka.TCP(broker)}
	resp, err := client.OffsetFetch(ctx, &kafka.OffsetFetchRequest{
		GroupID: group,
		Topics:  map[string][]int{topic: {partition}},
	})
	if err != nil || resp == nil {
		return 0
	}
	for _, parts := range resp.Topics[topic] {
		if parts.Partition == partition {
			return parts.CommittedOffset
		}
	}
	return 0
}
