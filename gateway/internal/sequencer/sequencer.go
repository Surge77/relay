// Package sequencer assigns per-conversation sequence numbers. It is the single
// source of message order: every value it returns for a conversation is strictly
// increasing and gap-free.
//
// Redis INCR is the hot path. The durable source of truth is Postgres: when the
// Redis counter is missing (cold start, restart, eviction) it is re-seeded from
// MAX(seq).
//
// Correctness note: Postgres history is written asynchronously by the durable
// stream consumer, so MAX(seq) can lag the highest seq already issued. Re-seeding
// from it is therefore safe to never regress ONLY when the Redis counter survives
// restarts — i.e. Redis is run with AOF persistence (the production assumption).
// Without AOF, a counter loss that also loses un-drained stream entries would
// reseed below recently-issued (but now-lost) seqs; those messages are gone
// either way, so surviving messages keep unique seqs, but the "never regresses"
// guarantee strictly depends on AOF. The dev miniredis broker has no AOF and is
// not durable across restarts by design.
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

// incrIfPresent atomically increments an existing counter, or returns -1 to
// signal a cold counter that must be recovered from the durable store. The hot
// path (counter present) is a single round-trip.
var incrIfPresent = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then
  return redis.call('INCR', KEYS[1])
end
return -1`)

// seedAndIncr seeds the counter from the durable high-water mark only if it is
// still absent, then increments — both under one Redis execution. Atomicity is
// what makes concurrent nodes safe: a racing seed can never overwrite a counter
// another node already advanced, so no seq is ever handed out twice.
var seedAndIncr = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  redis.call('SET', KEYS[1], ARGV[1])
end
return redis.call('INCR', KEYS[1])`)

// coldCounter is the sentinel incrIfPresent returns when the key is missing.
const coldCounter = -1

// Next returns the next sequence number for a conversation. The common path is a
// single atomic INCR; only a cold counter (first message, Redis restart, or
// eviction) pays the durable MAX(seq) recovery, after which seed+increment run
// atomically so concurrent nodes converge on one gap-free sequence.
func (r *Redis) Next(ctx context.Context, conversationID string) (int64, error) {
	k := key(conversationID)
	seq, err := incrIfPresent.Run(ctx, r.rdb, []string{k}).Int64()
	if err != nil {
		return 0, fmt.Errorf("seq incr: %w", err)
	}
	if seq != coldCounter {
		return seq, nil
	}

	max, err := r.store.MaxSeq(ctx, conversationID)
	if err != nil {
		return 0, fmt.Errorf("seq recovery: %w", err)
	}
	seq, err = seedAndIncr.Run(ctx, r.rdb, []string{k}, max).Int64()
	if err != nil {
		return 0, fmt.Errorf("seq seed: %w", err)
	}
	return seq, nil
}
