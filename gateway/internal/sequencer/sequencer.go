// Package sequencer assigns per-conversation sequence numbers. It is the single
// source of message order: every value it returns for a conversation is strictly
// increasing and gap-free.
//
// Redis INCR is the hot path. The durable source of truth is Postgres: when the
// Redis counter is missing (cold start, restart, eviction) it is re-seeded from
// MAX(seq) so the counter can never regress and hand out a duplicate seq.
package sequencer

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// SeqStore supplies the durable high-water mark for recovery. *store.Store
// implements it.
type SeqStore interface {
	MaxSeq(ctx context.Context, conversationID string) (int64, error)
}

// Redis is the production sequencer.
type Redis struct {
	rdb   *redis.Client
	store SeqStore
}

// NewRedis builds a sequencer over the given Redis client and durable store.
func NewRedis(rdb *redis.Client, store SeqStore) *Redis {
	return &Redis{rdb: rdb, store: store}
}

func key(conversationID string) string {
	return "conv:" + conversationID + ":seq"
}

// Next returns the next sequence number for a conversation. On a cold counter it
// seeds from the durable MAX(seq) under SETNX before incrementing, so concurrent
// nodes converge on a single monotonic sequence even across a Redis restart.
func (r *Redis) Next(ctx context.Context, conversationID string) (int64, error) {
	k := key(conversationID)
	exists, err := r.rdb.Exists(ctx, k).Result()
	if err != nil {
		return 0, fmt.Errorf("seq exists check: %w", err)
	}
	if exists == 0 {
		max, err := r.store.MaxSeq(ctx, conversationID)
		if err != nil {
			return 0, fmt.Errorf("seq recovery: %w", err)
		}
		// SETNX: only the first racer seeds; others no-op. Either way the
		// subsequent INCR is atomic and yields distinct increasing values.
		if err := r.rdb.SetNX(ctx, k, max, 0).Err(); err != nil {
			return 0, fmt.Errorf("seq seed: %w", err)
		}
	}
	seq, err := r.rdb.Incr(ctx, k).Result()
	if err != nil {
		return 0, fmt.Errorf("seq incr: %w", err)
	}
	return seq, nil
}
