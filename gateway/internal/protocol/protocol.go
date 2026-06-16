// Package protocol defines the JSON wire frames exchanged over the WebSocket.
// Both the gateway and the throwaway CLI client depend on these shapes, so they
// live in one place to keep the contract single-sourced.
package protocol

// Client→server frame types.
const (
	TypeSend      = "send"      // {client_msg_id, conversation_id, body}
	TypeSubscribe = "subscribe" // {conversation_id, last_acked_seq} — join + catch-up
	TypeTyping    = "typing"    // {conversation_id, state}
	TypeRead      = "read"      // {conversation_id, seq}
	TypePing      = "ping"
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
)

// Typing / presence states.
const (
	StateStart   = "start"
	StateStop    = "stop"
	StateOnline  = "online"
	StateOffline = "offline"
)

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
