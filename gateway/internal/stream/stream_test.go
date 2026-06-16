package stream

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/Surge77/relay/gateway/internal/model"
)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

type recordingSink struct {
	mu   sync.Mutex
	got  []model.Message
	fail map[string]bool // client_msg_id → fail once
}

func (r *recordingSink) handle(_ context.Context, m model.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail[m.ClientMsgID] {
		delete(r.fail, m.ClientMsgID) // fail once, then succeed
		return errors.New("transient sink failure")
	}
	r.got = append(r.got, m)
	return nil
}

func (r *recordingSink) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.got)
}

func appendN(t *testing.T, log *Log, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 1; i <= n; i++ {
		err := log.Persist(ctx, model.Message{
			ConversationID: "general", Seq: int64(i), SenderID: "alice",
			ClientMsgID: string(rune('a' + i - 1)), Body: "m", TS: int64(i),
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
}

func pendingCount(t *testing.T, rdb *redis.Client) int64 {
	t.Helper()
	p, err := rdb.XPending(context.Background(), StreamKey, Group).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	return p.Count
}

func TestConsumer_DrainsAndAcksAll(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()
	log := NewLog(rdb)
	sink := &recordingSink{fail: map[string]bool{}}
	c := NewConsumer(rdb, "node1", sink.handle)
	if err := c.EnsureGroup(ctx); err != nil {
		t.Fatalf("ensure group: %v", err)
	}
	appendN(t, log, 3)

	if _, err := c.readBatchCount(ctx, ">"); err != nil {
		t.Fatalf("read: %v", err)
	}
	if sink.count() != 3 {
		t.Fatalf("sink got %d, want 3", sink.count())
	}
	if pc := pendingCount(t, rdb); pc != 0 {
		t.Fatalf("pending = %d, want 0 (all acked)", pc)
	}
}

func TestConsumer_RedeliversOnSinkFailure(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()
	log := NewLog(rdb)
	sink := &recordingSink{fail: map[string]bool{"a": true}} // message "a" fails once
	c := NewConsumer(rdb, "node1", sink.handle)
	if err := c.EnsureGroup(ctx); err != nil {
		t.Fatalf("ensure group: %v", err)
	}
	appendN(t, log, 3)

	// First pass: "a" fails and stays pending; the other two ack.
	c.readBatchCount(ctx, ">")
	if pc := pendingCount(t, rdb); pc != 1 {
		t.Fatalf("pending after first pass = %d, want 1", pc)
	}
	// Reprocess pending: "a" now succeeds.
	if err := c.drainPending(ctx); err != nil {
		t.Fatalf("drain pending: %v", err)
	}
	if sink.count() != 3 {
		t.Fatalf("sink got %d, want 3 after redelivery", sink.count())
	}
	if pc := pendingCount(t, rdb); pc != 0 {
		t.Fatalf("pending = %d, want 0 after redelivery", pc)
	}
}

func TestConsumer_RestartReprocessesPending(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()
	log := NewLog(rdb)
	appendN(t, log, 2)

	// "Crashed" consumer: reads but its sink fails everything, leaving entries
	// pending for consumer name "node1".
	crashed := &recordingSink{fail: map[string]bool{"a": true, "b": true}}
	c1 := NewConsumer(rdb, "node1", crashed.handle)
	c1.EnsureGroup(ctx)
	c1.readBatchCount(ctx, ">")
	if pc := pendingCount(t, rdb); pc != 2 {
		t.Fatalf("pending after crash = %d, want 2", pc)
	}

	// Restart with the same consumer name: drainPending reclaims the in-flight
	// entries — no acknowledged message is lost across the restart.
	recovered := &recordingSink{fail: map[string]bool{}}
	c2 := NewConsumer(rdb, "node1", recovered.handle)
	if err := c2.drainPending(ctx); err != nil {
		t.Fatalf("drain pending: %v", err)
	}
	if recovered.count() != 2 {
		t.Fatalf("recovered sink got %d, want 2", recovered.count())
	}
	if pc := pendingCount(t, rdb); pc != 0 {
		t.Fatalf("pending = %d, want 0 after restart recovery", pc)
	}
}
