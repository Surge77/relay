package store

import (
	"context"
	"fmt"

	"github.com/Surge77/relay/gateway/internal/model"
)

// HistoryBefore returns up to limit messages with seq strictly less than
// beforeSeq (or the latest when beforeSeq is 0), ordered ascending — the
// backward scrollback query that complements the forward catch-up HistoryAfter.
func (s *Store) HistoryBefore(ctx context.Context, conversationID string, beforeSeq int64, limit int) ([]model.Message, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT conversation_id, seq, sender_id, client_msg_id, body, ts
		   FROM messages
		  WHERE conversation_id=$1 AND ($2 = 0 OR seq < $2)
		  ORDER BY seq DESC
		  LIMIT $3`,
		conversationID, beforeSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("history before: %w", err)
	}
	defer rows.Close()

	var desc []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ConversationID, &m.Seq, &m.SenderID, &m.ClientMsgID, &m.Body, &m.TS); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		desc = append(desc, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Reverse to ascending so callers render oldest→newest.
	for i, j := 0, len(desc)-1; i < j; i, j = i+1, j-1 {
		desc[i], desc[j] = desc[j], desc[i]
	}
	return desc, nil
}

// UnreadCount returns how many messages in a conversation the user has not read
// (highest seq minus their last_read_seq, floored at zero).
func (s *Store) UnreadCount(ctx context.Context, conversationID, userID string) (int64, error) {
	var unread int64
	err := s.pool.QueryRow(ctx,
		`SELECT GREATEST(
		    COALESCE((SELECT MAX(seq) FROM messages WHERE conversation_id=$1), 0) -
		    COALESCE((SELECT last_read_seq FROM memberships WHERE conversation_id=$1 AND user_id=$2), 0), 0)`,
		conversationID, userID).Scan(&unread)
	if err != nil {
		return 0, fmt.Errorf("unread count: %w", err)
	}
	return unread, nil
}
