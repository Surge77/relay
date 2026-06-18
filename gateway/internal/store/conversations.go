package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Surge77/relay/gateway/internal/model"
)

// CreateConversation inserts a conversation and makes the creator its owner, in
// one transaction so a conversation never exists without an owner.
func (s *Store) CreateConversation(ctx context.Context, c model.Conversation) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO conversations (id, kind, name, created_by) VALUES ($1,$2,NULLIF($3,''),$4)`,
			c.ID, c.Kind, c.Name, c.CreatedBy); err != nil {
			return fmt.Errorf("insert conversation: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO memberships (conversation_id, user_id, role) VALUES ($1,$2,'owner')`,
			c.ID, c.CreatedBy); err != nil {
			return fmt.Errorf("insert owner membership: %w", err)
		}
		return nil
	})
}

// GetOrCreateDM returns the direct-message conversation between two users,
// creating it (with both memberships) on first contact. The sorted dm_key makes
// this idempotent regardless of argument order.
func (s *Store) GetOrCreateDM(ctx context.Context, userA, userB string) (model.Conversation, error) {
	key := dmKey(userA, userB)
	var c model.Conversation
	err := s.pool.QueryRow(ctx,
		`SELECT id, kind, COALESCE(name,''), COALESCE(created_by,'') FROM conversations WHERE dm_key=$1`, key).
		Scan(&c.ID, &c.Kind, &c.Name, &c.CreatedBy)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Conversation{}, fmt.Errorf("find dm: %w", err)
	}

	c = model.Conversation{ID: "dm_" + key, Kind: "dm", CreatedBy: userA}
	err = s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO conversations (id, kind, created_by, dm_key) VALUES ($1,'dm',$2,$3)
			 ON CONFLICT (dm_key) DO NOTHING`, c.ID, userA, key); err != nil {
			return fmt.Errorf("insert dm: %w", err)
		}
		for _, u := range []string{userA, userB} {
			if _, err := tx.Exec(ctx,
				`INSERT INTO memberships (conversation_id, user_id) VALUES ($1,$2)
				 ON CONFLICT (conversation_id, user_id) DO NOTHING`, c.ID, u); err != nil {
				return fmt.Errorf("insert dm member: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return model.Conversation{}, err
	}
	return c, nil
}

// ListConversationsFor returns every conversation the user belongs to, with their
// unread count and the latest message, newest activity first. One query with
// lateral joins — no N+1.
func (s *Store) ListConversationsFor(ctx context.Context, userID string) ([]model.ConversationSummary, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.id, c.kind, COALESCE(c.name,''), COALESCE(c.created_by,''),
		        GREATEST(COALESCE(hw.max_seq,0) - m.last_read_seq, 0) AS unread,
		        lm.seq, lm.sender_id, lm.body, lm.ts
		   FROM memberships m
		   JOIN conversations c ON c.id = m.conversation_id
		   LEFT JOIN LATERAL (SELECT MAX(seq) AS max_seq FROM messages WHERE conversation_id=c.id) hw ON true
		   LEFT JOIN LATERAL (SELECT seq, sender_id, body, ts FROM messages
		                       WHERE conversation_id=c.id ORDER BY seq DESC LIMIT 1) lm ON true
		  WHERE m.user_id=$1
		  ORDER BY lm.ts DESC NULLS LAST`, userID)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	var out []model.ConversationSummary
	for rows.Next() {
		var cs model.ConversationSummary
		var seq, ts *int64
		var sender, body *string
		if err := rows.Scan(&cs.ID, &cs.Kind, &cs.Name, &cs.CreatedBy, &cs.UnreadCount,
			&seq, &sender, &body, &ts); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		if seq != nil {
			cs.LastMessage = &model.Message{
				ConversationID: cs.ID, Seq: *seq, SenderID: derefStr(sender), Body: derefStr(body), TS: derefInt(ts),
			}
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

// ConversationDetail returns a conversation and its members.
func (s *Store) ConversationDetail(ctx context.Context, conversationID string) (model.Conversation, []model.Member, error) {
	var c model.Conversation
	err := s.pool.QueryRow(ctx,
		`SELECT id, kind, COALESCE(name,''), COALESCE(created_by,'') FROM conversations WHERE id=$1`,
		conversationID).Scan(&c.ID, &c.Kind, &c.Name, &c.CreatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Conversation{}, nil, ErrNotFound
	}
	if err != nil {
		return model.Conversation{}, nil, fmt.Errorf("conversation detail: %w", err)
	}
	rows, err := s.pool.Query(ctx,
		`SELECT m.user_id, COALESCE(u.display_name,''), m.role
		   FROM memberships m JOIN users u ON u.id=m.user_id
		  WHERE m.conversation_id=$1 ORDER BY m.joined_at`, conversationID)
	if err != nil {
		return model.Conversation{}, nil, fmt.Errorf("members: %w", err)
	}
	defer rows.Close()
	var members []model.Member
	for rows.Next() {
		var mem model.Member
		if err := rows.Scan(&mem.UserID, &mem.DisplayName, &mem.Role); err != nil {
			return model.Conversation{}, nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, mem)
	}
	return c, members, rows.Err()
}

// MemberRole returns the user's role in a conversation, or ErrNotFound if they
// are not a member. Used for authorization on mutating endpoints.
func (s *Store) MemberRole(ctx context.Context, conversationID, userID string) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM memberships WHERE conversation_id=$1 AND user_id=$2`,
		conversationID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("member role: %w", err)
	}
	return role, nil
}

// RemoveMember removes a user from a conversation (also used for "leave").
func (s *Store) RemoveMember(ctx context.Context, conversationID, userID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM memberships WHERE conversation_id=$1 AND user_id=$2`, conversationID, userID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	return nil
}

// RenameConversation updates a conversation's display name.
func (s *Store) RenameConversation(ctx context.Context, conversationID, name string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET name=$2, updated_at=now() WHERE id=$1`, conversationID, name)
	if err != nil {
		return fmt.Errorf("rename conversation: %w", err)
	}
	return nil
}

// tx runs fn inside a transaction, committing on success and rolling back on error.
func (s *Store) tx(ctx context.Context, fn func(pgx.Tx) error) error {
	t, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(t); err != nil {
		_ = t.Rollback(ctx)
		return err
	}
	return t.Commit(ctx)
}

func dmKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + ":" + b
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefInt(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
