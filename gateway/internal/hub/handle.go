package hub

import (
	"context"
	"time"

	"github.com/Surge77/relay/gateway/internal/model"
	"github.com/Surge77/relay/gateway/internal/protocol"
	"github.com/Surge77/relay/gateway/internal/registry"
)

// OnConnect registers the connection and marks the user online, broadcasting
// presence to the conversations they belong to.
func (h *Hub) OnConnect(c registry.Client) {
	h.reg.Add(c)
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if err := h.presence.Online(ctx, c.UserID()); err == nil {
		h.broadcastPresence(ctx, c.UserID(), protocol.StateOnline)
	}
}

// OnDisconnect deregisters the connection. If the user has no remaining local
// connection, it clears presence and broadcasts offline.
func (h *Hub) OnDisconnect(c registry.Client) {
	h.reg.Remove(c)
	if h.reg.HasLocalMember(c.UserID()) {
		return // another tab is still open on this node
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if err := h.presence.Offline(ctx, c.UserID()); err == nil {
		h.broadcastPresence(ctx, c.UserID(), protocol.StateOffline)
	}
}

// OnFrame dispatches a validated inbound frame from a client.
func (h *Hub) OnFrame(ctx context.Context, c registry.Client, f protocol.Frame) {
	switch f.Type {
	case protocol.TypeSend:
		h.handleSend(ctx, c, f)
	case protocol.TypeSubscribe:
		h.handleSubscribe(ctx, c, f)
	case protocol.TypeTyping:
		h.handleTyping(ctx, c, f)
	case protocol.TypeRead:
		h.handleRead(ctx, c, f)
	case protocol.TypePing:
		octx, cancel := context.WithTimeout(ctx, opTimeout)
		_ = h.presence.Refresh(octx, c.UserID())
		cancel()
		c.Enqueue(protocol.Frame{Type: protocol.TypePong})
	default:
		sendErr(c, protocol.CodeBadFrame, "unknown frame type")
	}
}

// broadcastPresence publishes a presence change to every conversation the user
// belongs to, so co-members subscribed to those conversations see the dot flip.
func (h *Hub) broadcastPresence(ctx context.Context, userID, state string) {
	convs, err := h.store.ConversationsOf(ctx, userID)
	if err != nil {
		return
	}
	for _, conv := range convs {
		_ = h.fan.Publish(ctx, conv, protocol.Frame{
			Type:           protocol.TypePresence,
			ConversationID: conv,
			UserID:         userID,
			State:          state,
		})
	}
}

// handleRead records a read receipt and fans it out to the conversation.
func (h *Hub) handleRead(ctx context.Context, c registry.Client, f protocol.Frame) {
	if f.ConversationID == "" || f.Seq <= 0 {
		sendErr(c, protocol.CodeBadFrame, "read requires conversation_id and seq")
		return
	}
	if !h.authorize(ctx, c, f.ConversationID) {
		return
	}
	octx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	if err := h.store.SetLastRead(octx, f.ConversationID, c.UserID(), f.Seq); err != nil {
		sendErr(c, protocol.CodeInternal, "could not record read receipt")
		return
	}
	_ = h.fan.Publish(octx, f.ConversationID, protocol.Frame{
		Type:           protocol.TypeReceipt,
		ConversationID: f.ConversationID,
		UserID:         c.UserID(),
		Seq:            f.Seq,
	})
}

func (h *Hub) handleSend(ctx context.Context, c registry.Client, f protocol.Frame) {
	if f.ConversationID == "" || f.Body == "" || f.ClientMsgID == "" {
		sendErr(c, protocol.CodeBadFrame, "send requires conversation_id, client_msg_id, body")
		return
	}
	if !h.authorize(ctx, c, f.ConversationID) {
		return
	}

	octx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	seq, err := h.seq.Next(octx, f.ConversationID)
	if err != nil {
		sendErr(c, protocol.CodeInternal, "could not sequence message")
		return
	}
	msg := model.Message{
		ConversationID: f.ConversationID,
		Seq:            seq,
		SenderID:       c.UserID(),
		ClientMsgID:    f.ClientMsgID,
		Body:           f.Body,
		TS:             time.Now().UnixMilli(),
	}

	// Durable FIRST, then live fan-out: anything ever delivered live is therefore
	// always recoverable by reconnect catch-up.
	if err := h.persist.Persist(octx, msg); err != nil {
		sendErr(c, protocol.CodeInternal, "could not persist message")
		return
	}
	out := protocol.Frame{
		Type:           protocol.TypeMessage,
		ConversationID: msg.ConversationID,
		Seq:            msg.Seq,
		SenderID:       msg.SenderID,
		ClientMsgID:    msg.ClientMsgID,
		Body:           msg.Body,
		TS:             msg.TS,
	}
	if err := h.fan.Publish(octx, msg.ConversationID, out); err != nil {
		sendErr(c, protocol.CodeInternal, "could not deliver message")
		return
	}
	c.Enqueue(protocol.Frame{
		Type:           protocol.TypeAck,
		ClientMsgID:    msg.ClientMsgID,
		ConversationID: msg.ConversationID,
		Seq:            msg.Seq,
	})
}

func (h *Hub) handleSubscribe(ctx context.Context, c registry.Client, f protocol.Frame) {
	if f.ConversationID == "" {
		sendErr(c, protocol.CodeBadFrame, "subscribe requires conversation_id")
		return
	}
	if !h.authorize(ctx, c, f.ConversationID) {
		return
	}
	// Join live fan-out BEFORE replaying history so no message can slip through
	// the gap between replay and live; the client dedupes any overlap by seq.
	h.fan.EnsureSubscribed(f.ConversationID)
	h.reg.Subscribe(c.ID(), f.ConversationID)
	h.replayCatchUp(ctx, c, f.ConversationID, f.LastAckedSeq)
}

func (h *Hub) replayCatchUp(ctx context.Context, c registry.Client, conversationID string, afterSeq int64) {
	octx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	msgs, err := h.store.HistoryAfter(octx, conversationID, afterSeq, HistoryLimit)
	if err != nil {
		sendErr(c, protocol.CodeInternal, "could not load history")
		return
	}
	var last = afterSeq
	for _, m := range msgs {
		c.Enqueue(protocol.Frame{
			Type:           protocol.TypeMessage,
			ConversationID: m.ConversationID,
			Seq:            m.Seq,
			SenderID:       m.SenderID,
			ClientMsgID:    m.ClientMsgID,
			Body:           m.Body,
			TS:             m.TS,
		})
		last = m.Seq
	}
	c.Enqueue(protocol.Frame{Type: protocol.TypeCaughtUp, ConversationID: conversationID, Seq: last})
}

func (h *Hub) handleTyping(ctx context.Context, c registry.Client, f protocol.Frame) {
	if f.ConversationID == "" {
		return
	}
	if !h.authorize(ctx, c, f.ConversationID) {
		return
	}
	state := protocol.StateStop
	if f.State == protocol.StateStart {
		state = protocol.StateStart
	}
	octx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	_ = h.fan.Publish(octx, f.ConversationID, protocol.Frame{
		Type:           protocol.TypeTyping,
		ConversationID: f.ConversationID,
		UserID:         c.UserID(),
		State:          state,
	})
}

// authorize checks membership and reports a FORBIDDEN error to the client on
// failure. Returns true only when the user may act on the conversation.
func (h *Hub) authorize(ctx context.Context, c registry.Client, conversationID string) bool {
	octx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	ok, err := h.store.IsMember(octx, c.UserID(), conversationID)
	if err != nil {
		sendErr(c, protocol.CodeInternal, "authorization check failed")
		return false
	}
	if !ok {
		sendErr(c, protocol.CodeForbidden, "not a member of this conversation")
		return false
	}
	return true
}

func sendErr(c registry.Client, code, msg string) {
	c.Enqueue(protocol.Frame{Type: protocol.TypeError, Code: code, Message: msg})
}
