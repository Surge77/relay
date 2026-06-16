// Package model holds the cross-layer domain types shared by the store, hub, and
// stream packages. Kept dependency-free so any layer can import it.
package model

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
