// Package fanout routes messages across gateway nodes using Redis pub/sub. Each
// conversation maps to a channel; a node subscribes to a conversation's channel
// while it hosts a local member, so a message published on any node reaches the
// members hosted on every other node.
package fanout

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"

	"github.com/Surge77/relay/gateway/internal/protocol"
)

func channel(conversationID string) string { return "fanout:" + conversationID }

// Redis is the production Fanout. Delivery of inbound frames is handed to the
// deliver callback (set to Hub.DeliverLocal at startup).
type Redis struct {
	rdb     *redis.Client
	deliver func(conversationID string, f protocol.Frame)

	mu      sync.Mutex
	subs    map[string]context.CancelFunc
	baseCtx context.Context
}

// NewRedis builds a fan-out over the given client. The base context bounds all
// subscription goroutines; cancel it (or call Close) to stop them.
func NewRedis(ctx context.Context, rdb *redis.Client) *Redis {
	return &Redis{
		rdb:     rdb,
		subs:    make(map[string]context.CancelFunc),
		baseCtx: ctx,
	}
}

// SetDeliver wires the inbound-frame callback after construction, resolving the
// hub↔fanout circular dependency.
func (r *Redis) SetDeliver(deliver func(conversationID string, f protocol.Frame)) {
	r.deliver = deliver
}

// Publish broadcasts a frame to every node subscribed to the conversation,
// including this one (the local subscription loops it back for local delivery,
// so the hub never delivers directly).
func (r *Redis) Publish(ctx context.Context, conversationID string, f protocol.Frame) error {
	data, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshal frame: %w", err)
	}
	if err := r.rdb.Publish(ctx, channel(conversationID), data).Err(); err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}

// EnsureSubscribed makes this node start receiving the conversation's frames.
// Idempotent: a second call for the same conversation is a no-op.
func (r *Redis) EnsureSubscribed(conversationID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.subs[conversationID]; ok {
		return
	}
	ctx, cancel := context.WithCancel(r.baseCtx)
	pubsub := r.rdb.Subscribe(ctx, channel(conversationID))
	// Wait for the SUBSCRIBE to be acknowledged before returning. Without this,
	// a frame published in the window between Subscribe() and the consume loop
	// starting could be missed; the caller (handleSubscribe) then replays history
	// believing live delivery is already active. Confirming closes that gap.
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		cancel()
		return
	}
	r.subs[conversationID] = cancel
	go r.consume(ctx, conversationID, pubsub)
}

// Unsubscribe stops receiving a conversation's frames and releases the
// subscription goroutine + Redis connection. Idempotent.
func (r *Redis) Unsubscribe(conversationID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cancel, ok := r.subs[conversationID]; ok {
		cancel()
		delete(r.subs, conversationID)
	}
}

func (r *Redis) consume(ctx context.Context, conversationID string, pubsub *redis.PubSub) {
	defer pubsub.Close()
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var f protocol.Frame
			if err := json.Unmarshal([]byte(msg.Payload), &f); err != nil {
				continue
			}
			if r.deliver != nil {
				r.deliver(conversationID, f)
			}
		}
	}
}

// Close stops all subscription goroutines.
func (r *Redis) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, cancel := range r.subs {
		cancel()
	}
	r.subs = make(map[string]context.CancelFunc)
}
