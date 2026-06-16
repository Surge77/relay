package fanout_test

import (
	"context"
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

// TestNodeKillCatchUpNoLossNoDupe is the headline chaos test: a receiver's
// gateway dies mid-stream, more messages are sent while it is down, then the
// receiver reconnects to a different node and catches up. It must see every
// missed message exactly once, in order — zero loss, zero duplicate.
func TestNodeKillCatchUpNoLossNoDupe(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	store := devinfra.NewStore()
	store.AddMember("general", "alice")
	store.AddMember("general", "bob")
	seq := devinfra.NewSequencer()

	// nodeA hosts bob (the receiver); nodeB hosts alice (the sender).
	ctxA, cancelA := context.WithCancel(context.Background())
	nodeA := newChaosNode(ctxA, mr.Addr(), seq, store)
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	nodeB := newChaosNode(ctxB, mr.Addr(), seq, store)

	bob := &fakeClient{id: "cb", user: "bob"}
	alice := &fakeClient{id: "ca", user: "alice"}
	nodeA.hub.OnConnect(bob)
	nodeB.hub.OnConnect(alice)
	nodeA.hub.OnFrame(ctxA, bob, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general"})
	nodeB.hub.OnFrame(ctxB, alice, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general"})
	time.Sleep(50 * time.Millisecond)

	// Two messages delivered live to bob on node A.
	send(nodeB, ctxB, alice, "m1")
	send(nodeB, ctxB, alice, "m2")
	waitForMessages(t, bob, 2)

	// node A dies: bob's connection drops and the node's fan-out stops.
	nodeA.hub.OnDisconnect(bob)
	nodeA.fan.Close()
	cancelA()

	// While bob is offline, two more messages are sent and durably persisted.
	send(nodeB, ctxB, alice, "m3")
	send(nodeB, ctxB, alice, "m4")

	// bob reconnects to node B and catches up from his last acked seq (2).
	bob2 := &fakeClient{id: "cb2", user: "bob"}
	nodeB.hub.OnConnect(bob2)
	nodeB.hub.OnFrame(ctxB, bob2, protocol.Frame{
		Type: protocol.TypeSubscribe, ConversationID: "general", LastAckedSeq: 2,
	})

	msgs := waitForMessages(t, bob2, 2)
	if len(msgs) != 2 || msgs[0].Seq != 3 || msgs[1].Seq != 4 {
		t.Fatalf("catch-up = %+v, want exactly seq [3,4] (no loss, no dupe)", msgs)
	}
}

func newChaosNode(ctx context.Context, addr string, seq *devinfra.Sequencer, store *devinfra.Store) *node {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	reg := registry.New()
	fan := fanout.NewRedis(ctx, rdb)
	h := hub.New(reg, seq, store, fan, store, presence.Noop{})
	fan.SetDeliver(h.DeliverLocal)
	return &node{hub: h, fan: fan}
}

func send(n *node, ctx context.Context, c *fakeClient, body string) {
	n.hub.OnFrame(ctx, c, protocol.Frame{
		Type: protocol.TypeSend, ConversationID: "general", ClientMsgID: body, Body: body,
	})
}
