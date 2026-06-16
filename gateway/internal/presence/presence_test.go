package presence

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestPresence(t *testing.T) (*Redis, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return NewRedis(rdb), mr
}

func TestOnlineOffline(t *testing.T) {
	p, _ := newTestPresence(t)
	ctx := context.Background()

	if on, _ := p.IsOnline(ctx, "alice"); on {
		t.Fatal("alice should start offline")
	}
	if err := p.Online(ctx, "alice"); err != nil {
		t.Fatalf("Online: %v", err)
	}
	if on, _ := p.IsOnline(ctx, "alice"); !on {
		t.Fatal("alice should be online after Online")
	}
	if err := p.Offline(ctx, "alice"); err != nil {
		t.Fatalf("Offline: %v", err)
	}
	if on, _ := p.IsOnline(ctx, "alice"); on {
		t.Fatal("alice should be offline after Offline")
	}
}

func TestPresenceExpiresWithoutHeartbeat(t *testing.T) {
	p, mr := newTestPresence(t)
	ctx := context.Background()
	if err := p.Online(ctx, "bob"); err != nil {
		t.Fatalf("Online: %v", err)
	}
	// Fast-forward past the TTL without a refresh: the marker self-expires.
	mr.FastForward(TTL + time.Second)
	if on, _ := p.IsOnline(ctx, "bob"); on {
		t.Fatal("presence should expire without a heartbeat")
	}
}
