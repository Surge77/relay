// Package hub orchestrates the message path: it authorizes, sequences, durably
// persists, then fans out. It is transport-agnostic — it speaks to connections
// through the registry.Client interface and to infrastructure through the
// Sequencer, Store, and Fanout interfaces, so the same logic runs unchanged
// whether backed by in-memory fakes (tests / Phase 1) or Redis+Postgres.
package hub

import (
	"context"
	"time"

	"github.com/Surge77/relay/gateway/internal/model"
	"github.com/Surge77/relay/gateway/internal/protocol"
	"github.com/Surge77/relay/gateway/internal/registry"
)

// opTimeout caps every infrastructure call so a stalled dependency can never
// wedge a connection's read loop.
const opTimeout = 5 * time.Second

// Sequencer assigns the next per-conversation sequence number. Implementations
// must guarantee strictly-increasing, gap-free values per conversation.
type Sequencer interface {
	Next(ctx context.Context, conversationID string) (int64, error)
}

// Store is durable metadata + history.
type Store interface {
	IsMember(ctx context.Context, userID, conversationID string) (bool, error)
	HistoryAfter(ctx context.Context, conversationID string, afterSeq int64, limit int) ([]model.Message, error)
	ConversationsOf(ctx context.Context, userID string) ([]string, error)
	MembersOf(ctx context.Context, conversationID string) ([]string, error)
	SetLastRead(ctx context.Context, conversationID, userID string, seq int64) error
}

// Presence tracks online/offline state. Online is set on connect, refreshed by
// heartbeats, and cleared on disconnect. IsOnline backs the join-time snapshot so
// a subscriber learns who is already present, not only who connects afterwards.
type Presence interface {
	Online(ctx context.Context, userID string) error
	Refresh(ctx context.Context, userID string) error
	Offline(ctx context.Context, userID string) error
	IsOnline(ctx context.Context, userID string) (bool, error)
}

// Fanout routes a message to every node hosting an online member of a
// conversation. Publish sends; EnsureSubscribed makes this node start receiving
// the conversation's frames (delivered via the callback passed to the impl).
type Fanout interface {
	Publish(ctx context.Context, conversationID string, f protocol.Frame) error
	EnsureSubscribed(conversationID string)
	// Unsubscribe releases this node's subscription once no local connection
	// follows the conversation, preventing an unbounded goroutine/connection leak.
	Unsubscribe(conversationID string)
}

// Persister durably records a message before it is fanned out live. Returning
// nil means the message is recoverable on reconnect even if this node dies
// immediately after.
type Persister interface {
	Persist(ctx context.Context, m model.Message) error
}

// HistoryLimit caps a single catch-up replay to bound memory and latency.
const HistoryLimit = 500

// Hub wires the dependencies together. One per node.
type Hub struct {
	reg      *registry.Registry
	seq      Sequencer
	store    Store
	fan      Fanout
	persist  Persister
	presence Presence
}

// New constructs a Hub. The Fanout implementation must be configured to call
// Hub.DeliverLocal for frames it receives for this node.
func New(reg *registry.Registry, seq Sequencer, store Store, fan Fanout, persist Persister, presence Presence) *Hub {
	return &Hub{reg: reg, seq: seq, store: store, fan: fan, persist: persist, presence: presence}
}

// DeliverLocal pushes a frame to every local connection subscribed to the
// conversation. This is the callback the Fanout layer invokes for inbound
// cross-node frames; a slow client is skipped (its Enqueue returns false)
// rather than blocking the others.
func (h *Hub) DeliverLocal(conversationID string, f protocol.Frame) {
	for _, c := range h.reg.LocalSubscribers(conversationID) {
		c.Enqueue(f)
	}
}
