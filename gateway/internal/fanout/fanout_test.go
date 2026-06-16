package fanout_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/Surge77/relay/gateway/internal/devinfra"
	"github.com/Surge77/relay/gateway/internal/fanout"
	"github.com/Surge77/relay/gateway/internal/hub"
	"github.com/Surge77/relay/gateway/internal/presence"
	"github.com/Surge77/relay/gateway/internal/protocol"
	"github.com/Surge77/relay/gateway/internal/registry"
)

type fakeClient struct {
	id, user string
	mu       sync.Mutex
	got      []protocol.Frame
}

func (f *fakeClient) ID() string     { return f.id }
func (f *fakeClient) UserID() string { return f.user }
func (f *fakeClient) Enqueue(fr protocol.Frame) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, fr)
	return true
}
func (f *fakeClient) messages() []protocol.Frame {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []protocol.Frame
	for _, fr := range f.got {
		if fr.Type == protocol.TypeMessage {
			out = append(out, fr)
		}
	}
	return out
}

// node bundles the per-gateway state so the test can stand up two of them
// against the same Redis and shared metadata (as production would share Postgres).
type node struct {
	hub *hub.Hub
	fan *fanout.Redis
}

func newNode(ctx context.Context, addr string, seq *devinfra.Sequencer, store *devinfra.Store) *node {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	reg := registry.New()
	fan := fanout.NewRedis(ctx, rdb)
	h := hub.New(reg, seq, store, fan, store, presence.Noop{})
	fan.SetDeliver(h.DeliverLocal)
	return &node{hub: h, fan: fan}
}

func waitForMessages(t *testing.T, c *fakeClient, n int) []protocol.Frame {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if msgs := c.messages(); len(msgs) >= n {
			return msgs
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d messages, got %d", n, len(c.messages()))
	return nil
}

func TestCrossNodeFanoutInOrder(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Shared metadata across both nodes (stands in for shared Postgres + seq).
	store := devinfra.NewStore()
	store.AddMember("general", "alice")
	store.AddMember("general", "bob")
	seq := devinfra.NewSequencer()

	nodeA := newNode(ctx, mr.Addr(), seq, store)
	nodeB := newNode(ctx, mr.Addr(), seq, store)

	alice := &fakeClient{id: "ca", user: "alice"}
	bob := &fakeClient{id: "cb", user: "bob"}
	nodeA.hub.OnConnect(alice)
	nodeB.hub.OnConnect(bob)

	// Each client subscribes on its own node.
	nodeA.hub.OnFrame(ctx, alice, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general"})
	nodeB.hub.OnFrame(ctx, bob, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general"})
	time.Sleep(50 * time.Millisecond) // let subscriptions register

	// Alice (node A) sends three messages; bob (node B) must receive them in order.
	for _, body := range []string{"one", "two", "three"} {
		nodeA.hub.OnFrame(ctx, alice, protocol.Frame{
			Type: protocol.TypeSend, ConversationID: "general", ClientMsgID: body, Body: body,
		})
	}

	msgs := waitForMessages(t, bob, 3)
	if len(msgs) != 3 {
		t.Fatalf("bob got %d messages, want 3", len(msgs))
	}
	for i, want := range []string{"one", "two", "three"} {
		if msgs[i].Body != want || msgs[i].Seq != int64(i+1) {
			t.Fatalf("msg[%d] = {seq:%d body:%q}, want {seq:%d body:%q}", i, msgs[i].Seq, msgs[i].Body, i+1, want)
		}
	}
}
