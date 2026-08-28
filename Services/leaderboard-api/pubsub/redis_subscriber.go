// Package pubsub fans Redis leaderboard updates out to every pod's hub.
package pubsub

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Broadcaster receives serialized updates.
type Broadcaster interface{ Broadcast([]byte) }

// Subscriber relays the leaderboard:updates channel into the local hub.
type Subscriber struct {
	redis *redis.Client
	hub   Broadcaster
}

// New constructs the subscriber.
func New(rdb *redis.Client, hub Broadcaster) *Subscriber {
	return &Subscriber{redis: rdb, hub: hub}
}

// Run subscribes until ctx is done.
func (s *Subscriber) Run(ctx context.Context) {
	sub := s.redis.Subscribe(ctx, "leaderboard:updates")
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			s.hub.Broadcast([]byte(msg.Payload))
		}
	}
}
