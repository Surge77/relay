//go:build integration

// These tests require a live Postgres with migrations applied (POSTGRES_URL).
// Run with: go test -tags=integration ./internal/store/
package store

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/Surge77/relay/gateway/internal/model"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		t.Skip("POSTGRES_URL not set")
	}
	s, err := New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

// seedConversation creates a throwaway conversation + member so each test is
// isolated by a unique id.
func seedConversation(t *testing.T, s *Store) (conv, user string) {
	t.Helper()
	conv = "conv-" + uuid.NewString()
	user = "user-" + uuid.NewString()
	ctx := context.Background()
	_, err := s.pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1,$1)`, user)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO conversations (id) VALUES ($1)`, conv)
	if err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO memberships (conversation_id, user_id) VALUES ($1,$2)`, conv, user)
	if err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return conv, user
}

func TestIsMember(t *testing.T) {
	s := newStore(t)
	conv, user := seedConversation(t, s)
	ctx := context.Background()
	if ok, err := s.IsMember(ctx, user, conv); err != nil || !ok {
		t.Fatalf("IsMember(member) = %v,%v; want true,nil", ok, err)
	}
	if ok, _ := s.IsMember(ctx, "stranger", conv); ok {
		t.Fatal("IsMember(stranger) = true; want false")
	}
}

func TestPersistHistoryAndMaxSeq(t *testing.T) {
	s := newStore(t)
	conv, user := seedConversation(t, s)
	ctx := context.Background()

	for i := int64(1); i <= 3; i++ {
		err := s.Persist(ctx, model.Message{
			ConversationID: conv, Seq: i, SenderID: user,
			ClientMsgID: uuid.NewString(), Body: "m", TS: i,
		})
		if err != nil {
			t.Fatalf("persist %d: %v", i, err)
		}
	}

	msgs, err := s.HistoryAfter(ctx, conv, 1, 100)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(msgs) != 2 || msgs[0].Seq != 2 || msgs[1].Seq != 3 {
		t.Fatalf("history after 1 = %+v, want seq [2,3]", msgs)
	}

	max, err := s.MaxSeq(ctx, conv)
	if err != nil || max != 3 {
		t.Fatalf("MaxSeq = %d,%v; want 3,nil", max, err)
	}
}

func TestPersistIsIdempotent(t *testing.T) {
	s := newStore(t)
	conv, user := seedConversation(t, s)
	ctx := context.Background()
	cmid := uuid.NewString()
	m := model.Message{ConversationID: conv, Seq: 1, SenderID: user, ClientMsgID: cmid, Body: "x", TS: 1}

	if err := s.Persist(ctx, m); err != nil {
		t.Fatalf("first persist: %v", err)
	}
	// Replay of the same client_msg_id must not create a duplicate row.
	if err := s.Persist(ctx, m); err != nil {
		t.Fatalf("replay persist: %v", err)
	}
	msgs, _ := s.HistoryAfter(ctx, conv, 0, 100)
	if len(msgs) != 1 {
		t.Fatalf("history len = %d, want 1 after idempotent replay", len(msgs))
	}
}
