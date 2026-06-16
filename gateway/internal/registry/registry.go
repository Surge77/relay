// Package registry is a node-local index of live WebSocket connections and the
// conversations they are subscribed to. It holds no global or cross-node state —
// each gateway owns exactly one Registry for the sockets it terminates. Cross-node
// routing is the fan-out layer's job, not this one's.
package registry

import (
	"sync"

	"github.com/Surge77/relay/gateway/internal/protocol"
)

// Client is the minimal view of a connection the registry needs. ws.Conn
// implements it. Defined here (where it is used) to avoid importing ws and
// creating an import cycle.
type Client interface {
	ID() string
	UserID() string
	// Enqueue attempts a non-blocking send of a frame to the client. It returns
	// false if the send buffer is full or the connection is closing, so a slow
	// consumer can never block fan-out to others.
	Enqueue(protocol.Frame) bool
}

// Registry maps connections and conversation subscriptions for one node.
type Registry struct {
	mu     sync.RWMutex
	byConn map[string]Client              // connID → client
	byUser map[string]map[string]struct{} // userID → set of connID
	byConv map[string]map[string]struct{} // conversationID → set of connID
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{
		byConn: make(map[string]Client),
		byUser: make(map[string]map[string]struct{}),
		byConv: make(map[string]map[string]struct{}),
	}
}

// Add registers a connection. Call once when the socket is established.
func (r *Registry) Add(c Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byConn[c.ID()] = c
	addToSet(r.byUser, c.UserID(), c.ID())
}

// Remove deregisters a connection and all its conversation subscriptions. Call
// once when the socket closes.
func (r *Registry) Remove(c Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	connID := c.ID()
	delete(r.byConn, connID)
	removeFromSet(r.byUser, c.UserID(), connID)
	for conv, set := range r.byConv {
		if _, ok := set[connID]; ok {
			delete(set, connID)
			if len(set) == 0 {
				delete(r.byConv, conv)
			}
		}
	}
}

// Subscribe records that a connection is following a conversation, so locally
// fanned-out messages reach it.
func (r *Registry) Subscribe(connID, conversationID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byConn[connID]; !ok {
		return // unknown/closed connection; ignore
	}
	addToSet(r.byConv, conversationID, connID)
}

// LocalSubscribers returns the live clients on this node subscribed to a
// conversation. The returned slice is a snapshot safe to range over without the
// lock held.
func (r *Registry) LocalSubscribers(conversationID string) []Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	set := r.byConv[conversationID]
	out := make([]Client, 0, len(set))
	for connID := range set {
		if c, ok := r.byConn[connID]; ok {
			out = append(out, c)
		}
	}
	return out
}

// HasLocalMember reports whether the given user has any live connection on this
// node. Used to decide local offline-queue handling.
func (r *Registry) HasLocalMember(userID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byUser[userID]) > 0
}

// ConnCount returns the number of live connections on this node.
func (r *Registry) ConnCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byConn)
}

func addToSet(m map[string]map[string]struct{}, key, val string) {
	set := m[key]
	if set == nil {
		set = make(map[string]struct{})
		m[key] = set
	}
	set[val] = struct{}{}
}

func removeFromSet(m map[string]map[string]struct{}, key, val string) {
	set := m[key]
	if set == nil {
		return
	}
	delete(set, val)
	if len(set) == 0 {
		delete(m, key)
	}
}
