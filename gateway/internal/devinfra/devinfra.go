// Package devinfra provides in-memory implementations of the hub's Sequencer,
// Store, Persister, and Fanout interfaces. They back single-node dev runs (the
// Phase 1 CLI echo demo) and unit tests, with no Redis or Postgres required.
// They are NOT for production: state is per-process and lost on restart.
package devinfra

import (
	"context"
	"sort"
	"sync"

	"github.com/Surge77/relay/gateway/internal/model"
	"github.com/Surge77/relay/gateway/internal/protocol"
)

// Sequencer is an in-memory monotonic per-conversation counter.
type Sequencer struct {
	mu   sync.Mutex
	next map[string]int64
}

func NewSequencer() *Sequencer { return &Sequencer{next: make(map[string]int64)} }

func (s *Sequencer) Next(_ context.Context, conversationID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next[conversationID]++
	return s.next[conversationID], nil
}

// Store keeps memberships and message history in memory. It satisfies both
// hub.Store and hub.Persister.
type Store struct {
	mu       sync.RWMutex
	members  map[string]map[string]struct{} // conversationID → set of userID
	history  map[string][]model.Message     // conversationID → messages (sorted by seq)
	lastRead map[string]int64               // "conv|user" → last read seq
}

func NewStore() *Store {
	return &Store{
		members:  make(map[string]map[string]struct{}),
		history:  make(map[string][]model.Message),
		lastRead: make(map[string]int64),
	}
}

// AddMember grants a user access to a conversation (test/dev setup helper).
func (s *Store) AddMember(conversationID, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	set := s.members[conversationID]
	if set == nil {
		set = make(map[string]struct{})
		s.members[conversationID] = set
	}
	set[userID] = struct{}{}
}

func (s *Store) IsMember(_ context.Context, userID, conversationID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.members[conversationID][userID]
	return ok, nil
}

// ConversationsOf returns every conversation the user is a member of.
func (s *Store) ConversationsOf(_ context.Context, userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []string
	for conv, set := range s.members {
		if _, ok := set[userID]; ok {
			out = append(out, conv)
		}
	}
	return out, nil
}

// SetLastRead records a read receipt (in-memory; no-op persistence semantics).
func (s *Store) SetLastRead(_ context.Context, conversationID, userID string, seq int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastRead == nil {
		s.lastRead = make(map[string]int64)
	}
	s.lastRead[conversationID+"|"+userID] = seq
	return nil
}

// LastRead returns the recorded read cursor for a member (test/inspection helper).
func (s *Store) LastRead(conversationID, userID string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.lastRead[conversationID+"|"+userID]
	return v, ok
}

func (s *Store) Persist(_ context.Context, m model.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history[m.ConversationID] = append(s.history[m.ConversationID], m)
	return nil
}

func (s *Store) HistoryAfter(_ context.Context, conversationID string, afterSeq int64, limit int) ([]model.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := s.history[conversationID]
	out := make([]model.Message, 0)
	for _, m := range all {
		if m.Seq > afterSeq {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// LocalFanout delivers published frames synchronously to the local node via the
// deliver callback. Single-node only — there is no cross-node routing.
type LocalFanout struct {
	deliver func(conversationID string, f protocol.Frame)
}

func NewLocalFanout(deliver func(conversationID string, f protocol.Frame)) *LocalFanout {
	return &LocalFanout{deliver: deliver}
}

// SetDeliver wires the delivery callback after construction, resolving the
// hub↔fanout circular dependency at startup.
func (l *LocalFanout) SetDeliver(deliver func(conversationID string, f protocol.Frame)) {
	l.deliver = deliver
}

func (l *LocalFanout) Publish(_ context.Context, conversationID string, f protocol.Frame) error {
	l.deliver(conversationID, f)
	return nil
}

func (l *LocalFanout) EnsureSubscribed(string) {}
