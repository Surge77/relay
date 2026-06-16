package registry

import (
	"sync"
	"testing"

	"github.com/Surge77/relay/gateway/internal/protocol"
)

// fakeClient implements Client for tests, recording enqueued frames.
type fakeClient struct {
	id, user string
	mu       sync.Mutex
	got      []protocol.Frame
	full     bool // when true, Enqueue reports a full buffer
}

func (f *fakeClient) ID() string     { return f.id }
func (f *fakeClient) UserID() string { return f.user }
func (f *fakeClient) Enqueue(fr protocol.Frame) bool {
	if f.full {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, fr)
	return true
}

func TestSubscribeAndLocalSubscribers(t *testing.T) {
	r := New()
	a := &fakeClient{id: "c1", user: "u1"}
	b := &fakeClient{id: "c2", user: "u2"}
	r.Add(a)
	r.Add(b)
	r.Subscribe("c1", "conv-1")
	r.Subscribe("c2", "conv-1")
	r.Subscribe("c2", "conv-2")

	if got := len(r.LocalSubscribers("conv-1")); got != 2 {
		t.Fatalf("conv-1 subscribers = %d, want 2", got)
	}
	if got := len(r.LocalSubscribers("conv-2")); got != 1 {
		t.Fatalf("conv-2 subscribers = %d, want 1", got)
	}
}

func TestSubscribeUnknownConnIgnored(t *testing.T) {
	r := New()
	r.Subscribe("ghost", "conv-1")
	if got := len(r.LocalSubscribers("conv-1")); got != 0 {
		t.Fatalf("subscribers = %d, want 0 for unknown conn", got)
	}
}

func TestRemoveCleansSubscriptions(t *testing.T) {
	r := New()
	a := &fakeClient{id: "c1", user: "u1"}
	r.Add(a)
	r.Subscribe("c1", "conv-1")
	if !r.HasLocalMember("u1") {
		t.Fatal("u1 should be present")
	}
	r.Remove(a)
	if r.HasLocalMember("u1") {
		t.Fatal("u1 should be gone after Remove")
	}
	if got := len(r.LocalSubscribers("conv-1")); got != 0 {
		t.Fatalf("subscribers = %d, want 0 after Remove", got)
	}
	if r.ConnCount() != 0 {
		t.Fatalf("ConnCount = %d, want 0", r.ConnCount())
	}
}

func TestMultipleConnsPerUser(t *testing.T) {
	r := New()
	tab1 := &fakeClient{id: "c1", user: "u1"}
	tab2 := &fakeClient{id: "c2", user: "u1"}
	r.Add(tab1)
	r.Add(tab2)
	r.Remove(tab1)
	if !r.HasLocalMember("u1") {
		t.Fatal("u1 still has tab2 open; should remain present")
	}
}
