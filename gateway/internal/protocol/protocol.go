// Package protocol defines the JSON wire frames exchanged over the WebSocket.
// Both the gateway and the throwaway CLI client depend on these shapes, so they
// live in one place to keep the contract single-sourced.
package protocol

// Client→server frame types.
const (
	TypeSend      = "send"      // {client_msg_id, conversation_id, body, reply_to_seq?}
	TypeSubscribe = "subscribe" // {conversation_id, last_acked_seq} — join + catch-up
	TypeTyping    = "typing"    // {conversation_id, state}
	TypeRead      = "read"      // {conversation_id, seq}
	TypePing      = "ping"
	TypeEdit      = "edit"    // {conversation_id, seq, body} — author only
	TypeDelete    = "delete"  // {conversation_id, seq} — author only (soft delete)
	TypeReact     = "react"   // {conversation_id, seq, emoji}
	TypeUnreact   = "unreact" // {conversation_id, seq, emoji}
)

// Server→client frame types.
const (
	TypeAck      = "ack"      // {client_msg_id, conversation_id, seq}
	TypeMessage  = "message"  // {conversation_id, seq, sender_id, client_msg_id, body, ts}
	TypePresence = "presence" // {conversation_id, user_id, state}
	TypeReceipt  = "receipt"  // {conversation_id, user_id, seq}
	TypeCaughtUp = "caughtup" // {conversation_id, seq}
	TypePong     = "pong"
	TypeError    = "error" // {code, message}

	// Control-plane events, published by the REST API and delivered over the
	// realtime channel so connected clients react without polling. Conversation-
	// scoped events go to the conversation channel; user-targeted events (e.g.
	// "you were added") go to the recipient's per-user channel.
	TypeConversationCreated = "conversation_created" // {conversation_id, kind, name, actor_id}
	TypeConversationUpdated = "conversation_updated" // {conversation_id, name, actor_id}
	TypeMemberAdded         = "member_added"         // {conversation_id, user_id, actor_id}
	TypeMemberRemoved       = "member_removed"       // {conversation_id, user_id, actor_id}

	TypeMessageEdited   = "message_edited"   // {conversation_id, seq, body, ts}
	TypeMessageDeleted  = "message_deleted"  // {conversation_id, seq}
	TypeReactionAdded   = "reaction_added"   // {conversation_id, seq, user_id, emoji}
	TypeReactionRemoved = "reaction_removed" // {conversation_id, seq, user_id, emoji}
)

// Typing / presence states.
const (
	StateStart   = "start"
	StateStop    = "stop"
	StateOnline  = "online"
	StateOffline = "offline"
)

// UserChannel is the fan-out routing key for a user's personal event channel
// (member-added, etc.). The gateway subscribes each connection to it on connect;
// the REST control plane publishes user-targeted frames to it.
func UserChannel(userID string) string { return "user:" + userID }

// Frame is the envelope for every message in both directions. Unused fields are
// omitted so the same struct serves all frame types without per-type structs.
type Frame struct {
	Type           string `json:"type"`
	ClientMsgID    string `json:"client_msg_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	Body           string `json:"body,omitempty"`
	Seq            int64  `json:"seq,omitempty"`
	LastAckedSeq   int64  `json:"last_acked_seq,omitempty"`
	SenderID       string `json:"sender_id,omitempty"`
	UserID         string `json:"user_id,omitempty"`
	State          string `json:"state,omitempty"`
	TS             int64  `json:"ts,omitempty"`
	Code           string `json:"code,omitempty"`
	Message        string `json:"message,omitempty"`
	// Control-plane event fields.
	Kind    string `json:"kind,omitempty"`
	Name    string `json:"name,omitempty"`
	ActorID string `json:"actor_id,omitempty"`
	// Message-feature fields.
	ReplyToSeq int64  `json:"reply_to_seq,omitempty"`
	Emoji      string `json:"emoji,omitempty"`
}

// Error codes returned to clients (user-safe, no internal detail).
const (
	CodeBadFrame     = "BAD_FRAME"
	CodeUnauthorized = "UNAUTHORIZED"
	CodeForbidden    = "FORBIDDEN"
	CodeTooLarge     = "TOO_LARGE"
	CodeRateLimited  = "RATE_LIMITED"
	CodeInternal     = "INTERNAL"
)
