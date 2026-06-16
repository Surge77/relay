package sequencer

import (
	"context"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// fakeStore returns a fixed durable high-water mark.
type fakeStore struct {
	mu  sync.Mutex
	max int64
}

func (f *fakeStore) MaxSeq(context.Context, string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.max, nil
}

func newTestSequencer(t *testing.T, max int64) (*Redis, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return NewRedis(rdb, &fakeStore{max: max}), mr
}

func TestNext_StartsFromOneOnEmpty(t *testing.T) {
	s, _ := newTestSequencer(t, 0)
	got, err := s.Next(context.Background(), "c1")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got != 1 {
		t.Fatalf("first seq = %d, want 1", got)
	}
}

func TestNext_SeedsFromDurableMax(t *testing.T) {
	s, _ := newTestSequencer(t, 42)
	got, err := s.Next(context.Background(), "c1")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got != 43 {
		t.Fatalf("seq = %d, want 43 (seeded from MAX(seq)=42)", got)
	}
}

func TestNext_RecoversAfterRedisFlush(t *testing.T) {
	store := &fakeStore{max: 0}
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	s := NewRedis(rdb, store)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		s.Next(ctx, "c1")
	}
	// Simulate persisted state, then a Redis loss.
	store.mu.Lock()
	store.max = 5
	store.mu.Unlock()
	mr.FlushAll()

	got, err := s.Next(ctx, "c1")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got != 6 {
		t.Fatalf("post-flush seq = %d, want 6 (no regression)", got)
	}
}

// TestNext_ConcurrentIsGapFree is the property test: under concurrent producers
// the sequence is strictly increasing and gap-free — exactly {1..N}, no dupes.
func TestNext_ConcurrentIsGapFree(t *testing.T) {
	s, _ := newTestSequencer(t, 0)
	const n = 500
	ctx := context.Background()

	var mu sync.Mutex
	seen := make(map[int64]int)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			seq, err := s.Next(ctx, "c1")
			if err != nil {
				t.Errorf("Next: %v", err)
				return
			}
			mu.Lock()
			seen[seq]++
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(seen) != n {
		t.Fatalf("distinct seqs = %d, want %d", len(seen), n)
	}
	for i := int64(1); i <= n; i++ {
		if seen[i] != 1 {
			t.Fatalf("seq %d appeared %d times, want exactly 1 (gap or dupe)", i, seen[i])
		}
	}
}
