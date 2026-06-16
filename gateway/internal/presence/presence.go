// Package presence tracks who is online using short-lived Redis keys. A key is
// set when a user connects and refreshed by client heartbeats; if a node dies
// without cleaning up, the TTL expires the key automatically, so presence is
// self-healing and needs no cross-node coordination.
package presence

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TTL is how long an online marker survives without a heartbeat. Clients should
// ping at well under this interval.
const TTL = 30 * time.Second

func key(userID string) string { return "presence:" + userID }

// Redis is the production presence tracker.
type Redis struct {
	rdb *redis.Client
}

func NewRedis(rdb *redis.Client) *Redis { return &Redis{rdb: rdb} }

// Online marks a user online with a fresh TTL.
func (r *Redis) Online(ctx context.Context, userID string) error {
	if err := r.rdb.Set(ctx, key(userID), "1", TTL).Err(); err != nil {
		return fmt.Errorf("presence online: %w", err)
	}
	return nil
}

// Refresh extends the TTL on a heartbeat. Equivalent to Online but named for
// intent at the call site.
func (r *Redis) Refresh(ctx context.Context, userID string) error {
	return r.Online(ctx, userID)
}

// Offline clears the marker on a clean disconnect.
func (r *Redis) Offline(ctx context.Context, userID string) error {
	if err := r.rdb.Del(ctx, key(userID)).Err(); err != nil {
		return fmt.Errorf("presence offline: %w", err)
	}
	return nil
}

// IsOnline reports whether a user currently has a live marker.
func (r *Redis) IsOnline(ctx context.Context, userID string) (bool, error) {
	n, err := r.rdb.Exists(ctx, key(userID)).Result()
	if err != nil {
		return false, fmt.Errorf("presence check: %w", err)
	}
	return n > 0, nil
}

// Noop is a presence tracker that does nothing — used by the in-memory dev mode.
type Noop struct{}

func (Noop) Online(context.Context, string) error  { return nil }
func (Noop) Refresh(context.Context, string) error { return nil }
func (Noop) Offline(context.Context, string) error { return nil }
