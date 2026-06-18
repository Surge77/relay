// Package presence tracks who is online using short-lived Redis keys. A key is
// set when a user connects and refreshed by client heartbeats; if a node dies
// without cleaning up, the TTL expires the key automatically, so presence is
// self-healing and needs no cross-node coordination.
package presence

import (
	"context"
	"fmt"
	"sync"
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

// Noop is a presence tracker that records nothing. Retained for tests that do
// not exercise presence; IsOnline always reports false.
type Noop struct{}

func (Noop) Online(context.Context, string) error           { return nil }
func (Noop) Refresh(context.Context, string) error          { return nil }
func (Noop) Offline(context.Context, string) error          { return nil }
func (Noop) IsOnline(context.Context, string) (bool, error) { return false, nil }

// Memory is a process-local presence tracker for the single-node in-memory dev
// mode. It is correct only because every dev connection lands on the one node;
// multi-node deployments must use Redis so presence is visible across nodes.
type Memory struct {
	mu     sync.RWMutex
	online map[string]struct{}
}

func NewMemory() *Memory { return &Memory{online: make(map[string]struct{})} }

func (m *Memory) Online(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.online[userID] = struct{}{}
	return nil
}

func (m *Memory) Refresh(ctx context.Context, userID string) error { return m.Online(ctx, userID) }

func (m *Memory) Offline(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.online, userID)
	return nil
}

func (m *Memory) IsOnline(_ context.Context, userID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.online[userID]
	return ok, nil
}
