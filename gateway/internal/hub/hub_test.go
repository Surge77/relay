package hub

import (
	"context"
	"sync"
	"testing"

	"github.com/Surge77/relay/gateway/internal/devinfra"
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
func (f *fakeClient) frames() []protocol.Frame {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]protocol.Frame(nil), f.got...)
}

// newTestHub wires a hub on in-memory infra with the given members seeded into
// one conversation.
func newTestHub(conv string, users ...string) (*Hub, *devinfra.Store) {
	reg := registry.New()
	store := devinfra.NewStore()
	for _, u := range users {
		store.AddMember(conv, u)
	}
	lf := devinfra.NewLocalFanout(nil)
	h := New(reg, devinfra.NewSequencer(), store, lf, store, presence.Noop{})
	lf.SetDeliver(h.DeliverLocal)
	return h, store
}

func framesOfType(fs []protocol.Frame, t string) []protocol.Frame {
	var out []protocol.Frame
	for _, f := range fs {
		if f.Type == t {
			out = append(out, f)
		}
	}
	return out
}

func TestSend_FansOutAndAcks(t *testing.T) {
	ctx := context.Background()
	h, _ := newTestHub("general", "alice", "bob")
	alice := &fakeClient{id: "ca", user: "alice"}
	bob := &fakeClient{id: "cb", user: "bob"}
	h.OnConnect(alice)
	h.OnConnect(bob)
	h.OnFrame(ctx, alice, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general"})
	h.OnFrame(ctx, bob, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general"})

	h.OnFrame(ctx, alice, protocol.Frame{
		Type: protocol.TypeSend, ConversationID: "general", ClientMsgID: "m1", Body: "hi",
	})

	if acks := framesOfType(alice.frames(), protocol.TypeAck); len(acks) != 1 || acks[0].Seq != 1 {
		t.Fatalf("alice acks = %+v, want one ack with seq 1", acks)
	}
	bobMsgs := framesOfType(bob.frames(), protocol.TypeMessage)
	if len(bobMsgs) != 1 || bobMsgs[0].Body != "hi" || bobMsgs[0].Seq != 1 {
		t.Fatalf("bob messages = %+v, want one 'hi' at seq 1", bobMsgs)
	}
}

func TestSend_NonMemberForbidden(t *testing.T) {
	ctx := context.Background()
	h, _ := newTestHub("general", "alice")
	mallory := &fakeClient{id: "cm", user: "mallory"}
	h.OnConnect(mallory)
	h.OnFrame(ctx, mallory, protocol.Frame{
		Type: protocol.TypeSend, ConversationID: "general", ClientMsgID: "x", Body: "hi",
	})
	errs := framesOfType(mallory.frames(), protocol.TypeError)
	if len(errs) != 1 || errs[0].Code != protocol.CodeForbidden {
		t.Fatalf("expected one FORBIDDEN error, got %+v", mallory.frames())
	}
}

func TestRead_PersistsAndFansOutReceipt(t *testing.T) {
	ctx := context.Background()
	h, store := newTestHub("general", "alice", "bob")
	alice := &fakeClient{id: "ca", user: "alice"}
	bob := &fakeClient{id: "cb", user: "bob"}
	h.OnConnect(alice)
	h.OnConnect(bob)
	h.OnFrame(ctx, alice, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general"})
	h.OnFrame(ctx, bob, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general"})
	h.OnFrame(ctx, alice, protocol.Frame{Type: protocol.TypeSend, ConversationID: "general", ClientMsgID: "m1", Body: "hi"})

	h.OnFrame(ctx, bob, protocol.Frame{Type: protocol.TypeRead, ConversationID: "general", Seq: 1})

	receipts := framesOfType(alice.frames(), protocol.TypeReceipt)
	if len(receipts) != 1 || receipts[0].UserID != "bob" || receipts[0].Seq != 1 {
		t.Fatalf("alice receipts = %+v, want one from bob at seq 1", receipts)
	}
	if seq, _ := store.LastRead("general", "bob"); seq != 1 {
		t.Fatalf("stored last read = %d, want 1", seq)
	}
}

func TestConnect_BroadcastsPresence(t *testing.T) {
	ctx := context.Background()
	h, _ := newTestHub("general", "alice", "carol")
	alice := &fakeClient{id: "ca", user: "alice"}
	h.OnConnect(alice)
	h.OnFrame(ctx, alice, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general"})

	// carol connecting should fan a presence(online) frame to general subscribers.
	carol := &fakeClient{id: "cc", user: "carol"}
	h.OnConnect(carol)

	pres := framesOfType(alice.frames(), protocol.TypePresence)
	if len(pres) == 0 || pres[len(pres)-1].UserID != "carol" || pres[len(pres)-1].State != protocol.StateOnline {
		t.Fatalf("alice presence frames = %+v, want carol online", pres)
	}
}

func TestSubscribe_ReplaysHistoryInOrder(t *testing.T) {
	ctx := context.Background()
	h, _ := newTestHub("general", "alice", "bob")
	alice := &fakeClient{id: "ca", user: "alice"}
	h.OnConnect(alice)
	h.OnFrame(ctx, alice, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general"})
	for _, b := range []string{"a", "b", "c"} {
		h.OnFrame(ctx, alice, protocol.Frame{Type: protocol.TypeSend, ConversationID: "general", ClientMsgID: b, Body: b})
	}

	// New connection catches up from seq 1: should replay seq 2 and 3 only.
	bob := &fakeClient{id: "cb", user: "bob"}
	h.OnConnect(bob)
	h.OnFrame(ctx, bob, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general", LastAckedSeq: 1})

	msgs := framesOfType(bob.frames(), protocol.TypeMessage)
	if len(msgs) != 2 || msgs[0].Seq != 2 || msgs[1].Seq != 3 {
		t.Fatalf("catch-up replay = %+v, want seq [2,3]", msgs)
	}
	caught := framesOfType(bob.frames(), protocol.TypeCaughtUp)
	if len(caught) != 1 || caught[0].Seq != 3 {
		t.Fatalf("caughtup = %+v, want seq 3", caught)
	}
}
