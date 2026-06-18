// Package model holds the cross-layer domain types shared by the store, hub, and
// stream packages. Kept dependency-free so any layer can import it.
package model

import "time"

// Message is a single chat message as it lives in history. Seq is the
// per-conversation monotonic sequence assigned by the sequencer; it is the sole
// ordering and dedupe key.
type Message struct {
	ConversationID string
	Seq            int64
	SenderID       string
	ClientMsgID    string
	Body           string
	TS             int64 // unix milliseconds
}

// User is an account. Email and PasswordHash are empty for the legacy dev seed
// users (alice/bob/carol) that never use the credential login flow.
type User struct {
	ID           string
	Email        string
	DisplayName  string
	PasswordHash string
	AvatarURL    string
	StatusText   string
}

// RefreshToken is a stored session-refresh record. Only the hash of the opaque
// token is persisted; RevokedAt is non-nil once the token is rotated or revoked.
type RefreshToken struct {
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// Conversation is a channel, group, or direct message.
type Conversation struct {
	ID        string
	Kind      string // channel | dm | group
	Name      string
	CreatedBy string
}

// Member is a participant in a conversation.
type Member struct {
	UserID      string
	DisplayName string
	Role        string // owner | admin | member
}

// ConversationSummary is a conversation in a user's list with their unread count
// and the latest message preview (nil when the conversation has no messages).
type ConversationSummary struct {
	Conversation
	UnreadCount int64
	LastMessage *Message
}

// Attachment is an uploaded file's metadata; the bytes live in the Storage
// backend under StorageKey.
type Attachment struct {
	ID             string
	ConversationID string
	UploaderID     string
	Filename       string
	ContentType    string
	StorageKey     string
	SizeBytes      int64
	Width          int
	Height         int
}
