// Package store is the Postgres-backed metadata and history layer. It implements
// the hub's Store and Persister interfaces and the sequencer's MaxSeq lookup.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Surge77/relay/gateway/internal/model"
)

// Store wraps a pgx connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// New opens a connection pool to Postgres and verifies connectivity.
func New(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping verifies database connectivity for readiness checks.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// IsMember reports whether a user belongs to a conversation. Authorization for
// every SEND/READ flows through here — membership is never trusted from the
// client.
func (s *Store) IsMember(ctx context.Context, userID, conversationID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM memberships WHERE conversation_id=$1 AND user_id=$2)`,
		conversationID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("membership check: %w", err)
	}
	return exists, nil
}

// ConversationsOf returns every conversation the user belongs to. Used to scope
// presence broadcasts to the conversations a connecting user participates in.
func (s *Store) ConversationsOf(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT conversation_id FROM memberships WHERE user_id=$1`, userID)
	if err != nil {
		return nil, fmt.Errorf("conversations of: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// MembersOf returns every user belonging to a conversation. Used to build the
// join-time presence snapshot a subscriber receives.
func (s *Store) MembersOf(ctx context.Context, conversationID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_id FROM memberships WHERE conversation_id=$1`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("members of: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// AddMember adds a user to a conversation, idempotently. Roles and invite flows
// arrive in the conversation-management phase; for now new signups auto-join the
// seeded "general" channel so the chat works end-to-end.
func (s *Store) AddMember(ctx context.Context, conversationID, userID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO memberships (conversation_id, user_id) VALUES ($1, $2)
		 ON CONFLICT (conversation_id, user_id) DO NOTHING`,
		conversationID, userID)
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

// SetLastRead advances a member's read cursor, never moving it backwards.
func (s *Store) SetLastRead(ctx context.Context, conversationID, userID string, seq int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE memberships SET last_read_seq=$3
		  WHERE conversation_id=$1 AND user_id=$2 AND last_read_seq < $3`,
		conversationID, userID, seq)
	if err != nil {
		return fmt.Errorf("set last read: %w", err)
	}
	return nil
}

// Persist writes a message to history. It is idempotent: a replay of the same
// (conversation, sender, client_msg_id) is silently ignored, so at-least-once
// delivery never duplicates a row.
func (s *Store) Persist(ctx context.Context, m model.Message) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO messages (conversation_id, seq, sender_id, client_msg_id, body, ts)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (conversation_id, sender_id, client_msg_id) DO NOTHING`,
		m.ConversationID, m.Seq, m.SenderID, m.ClientMsgID, m.Body, m.TS,
	)
	if err != nil {
		return fmt.Errorf("persist message: %w", err)
	}
	return nil
}

// HistoryAfter returns up to limit messages with seq strictly greater than
// afterSeq, ordered by seq — the catch-up replay query.
func (s *Store) HistoryAfter(ctx context.Context, conversationID string, afterSeq int64, limit int) ([]model.Message, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT conversation_id, seq, sender_id, client_msg_id, body, ts
		   FROM messages
		  WHERE conversation_id=$1 AND seq>$2
		  ORDER BY seq ASC
		  LIMIT $3`,
		conversationID, afterSeq, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("history query: %w", err)
	}
	defer rows.Close()

	out := make([]model.Message, 0)
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ConversationID, &m.Seq, &m.SenderID, &m.ClientMsgID, &m.Body, &m.TS); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MaxSeq returns the highest seq persisted for a conversation, or 0 if none. The
// sequencer uses this to recover its counter after a Redis restart.
func (s *Store) MaxSeq(ctx context.Context, conversationID string) (int64, error) {
	var max int64
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq),0) FROM messages WHERE conversation_id=$1`,
		conversationID,
	).Scan(&max)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("max seq: %w", err)
	}
	return max, nil
}
