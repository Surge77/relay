package hub

import (
	"context"
	"log"
	"time"

	"github.com/Surge77/relay/gateway/internal/model"
	"github.com/Surge77/relay/gateway/internal/protocol"
	"github.com/Surge77/relay/gateway/internal/registry"
)

// OnConnect registers the connection and marks the user online, broadcasting
// presence to the conversations they belong to.
func (h *Hub) OnConnect(c registry.Client) {
	h.reg.Add(c)
	// Subscribe this connection to its per-user event channel so control-plane
	// events (added to a conversation, etc.) published by the REST API reach it.
	userChan := protocol.UserChannel(c.UserID())
	h.fan.EnsureSubscribed(userChan)
	h.reg.Subscribe(c.ID(), userChan)

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if err := h.presence.Online(ctx, c.UserID()); err == nil {
		h.broadcastPresence(ctx, c.UserID(), protocol.StateOnline)
	}
}

// presenceOfflineGrace is how long a user's last connection may be gone before
// we declare them offline. It absorbs brief reconnects (a network blip, a dev
// double-mount) so the presence dot doesn't flap online→offline→online on every
// reconnect — the offline only fires if no new connection arrives in time.
const presenceOfflineGrace = 1500 * time.Millisecond

// OnDisconnect deregisters the connection. If the user has no remaining local
// connection, it schedules a grace-delayed offline so a fast reconnect keeps the
// user continuously online.
func (h *Hub) OnDisconnect(c registry.Client) {
	emptied := h.reg.Remove(c)
	// Release fan-out subscriptions for conversations no local client follows.
	for _, conv := range emptied {
		h.fan.Unsubscribe(conv)
	}
	if h.reg.HasLocalMember(c.UserID()) {
		return // another tab is still open on this node
	}
	go h.goOfflineAfterGrace(c.UserID())
}

// goOfflineAfterGrace clears presence only if the user is still gone after the
// grace window — i.e. it was a real disconnect, not a reconnect in flight.
func (h *Hub) goOfflineAfterGrace(userID string) {
	time.Sleep(presenceOfflineGrace)
	if h.reg.HasLocalMember(userID) {
		return // reconnected within the grace window; stay online
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()
	if err := h.presence.Offline(ctx, userID); err == nil {
		h.broadcastPresence(ctx, userID, protocol.StateOffline)
	}
	// Record last-seen off the disconnect path; a failure must never matter here.
	if h.lastSeen != nil {
		if err := h.lastSeen.TouchLastSeen(ctx, userID); err != nil {
			log.Printf("touch last seen for %s: %v", userID, err)
		}
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
	case protocol.TypeEdit:
		h.handleEdit(ctx, c, f)
	case protocol.TypeDelete:
		h.handleDelete(ctx, c, f)
	case protocol.TypeReact:
		h.handleReaction(ctx, c, f, true)
	case protocol.TypeUnreact:
		h.handleReaction(ctx, c, f, false)
	case protocol.TypePing:
		octx, cancel := context.WithTimeout(ctx, opTimeout)
		if err := h.presence.Refresh(octx, c.UserID()); err != nil {
			log.Printf("presence refresh for %s: %v", c.UserID(), err)
		}
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
		if err := h.fan.Publish(ctx, conv, protocol.Frame{
			Type:           protocol.TypePresence,
			ConversationID: conv,
			UserID:         userID,
			State:          state,
		}); err != nil {
			log.Printf("publish presence %s to %s: %v", userID, conv, err)
		}
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
	if err := h.fan.Publish(octx, f.ConversationID, protocol.Frame{
		Type:           protocol.TypeReceipt,
		ConversationID: f.ConversationID,
		UserID:         c.UserID(),
		Seq:            f.Seq,
	}); err != nil {
		log.Printf("publish receipt to %s: %v", f.ConversationID, err)
	}
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
	h.sendPresenceSnapshot(ctx, c, f.ConversationID)
	h.replayCatchUp(ctx, c, f.ConversationID, f.LastAckedSeq)
}

// sendPresenceSnapshot tells a just-subscribed client which members are already
// online. Without it a client would only ever learn about users who connect
// AFTER it — so a peer already present (including itself) would show offline.
func (h *Hub) sendPresenceSnapshot(ctx context.Context, c registry.Client, conversationID string) {
	octx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	members, err := h.store.MembersOf(octx, conversationID)
	if err != nil {
		return // snapshot is best-effort; live presence events still flow
	}
	for _, userID := range members {
		online, err := h.presence.IsOnline(octx, userID)
		if err != nil || !online {
			continue
		}
		c.Enqueue(protocol.Frame{
			Type:           protocol.TypePresence,
			ConversationID: conversationID,
			UserID:         userID,
			State:          protocol.StateOnline,
		})
	}
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

// handleEdit updates a message body (author only) and fans out message_edited.
func (h *Hub) handleEdit(ctx context.Context, c registry.Client, f protocol.Frame) {
	if f.ConversationID == "" || f.Seq <= 0 || f.Body == "" {
		sendErr(c, protocol.CodeBadFrame, "edit requires conversation_id, seq, body")
		return
	}
	if !h.authorize(ctx, c, f.ConversationID) {
		return
	}
	octx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	if err := h.store.EditMessage(octx, f.ConversationID, f.Seq, c.UserID(), f.Body); err != nil {
		sendErr(c, protocol.CodeForbidden, "cannot edit this message")
		return
	}
	_ = h.fan.Publish(octx, f.ConversationID, protocol.Frame{
		Type: protocol.TypeMessageEdited, ConversationID: f.ConversationID,
		Seq: f.Seq, Body: f.Body, TS: time.Now().UnixMilli(),
	})
}

// handleDelete soft-deletes a message (author only) and fans out a tombstone.
func (h *Hub) handleDelete(ctx context.Context, c registry.Client, f protocol.Frame) {
	if f.ConversationID == "" || f.Seq <= 0 {
		sendErr(c, protocol.CodeBadFrame, "delete requires conversation_id and seq")
		return
	}
	if !h.authorize(ctx, c, f.ConversationID) {
		return
	}
	octx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	if err := h.store.SoftDeleteMessage(octx, f.ConversationID, f.Seq, c.UserID()); err != nil {
		sendErr(c, protocol.CodeForbidden, "cannot delete this message")
		return
	}
	_ = h.fan.Publish(octx, f.ConversationID, protocol.Frame{
		Type: protocol.TypeMessageDeleted, ConversationID: f.ConversationID, Seq: f.Seq,
	})
}

// handleReaction adds or removes a reaction and fans out the change.
func (h *Hub) handleReaction(ctx context.Context, c registry.Client, f protocol.Frame, add bool) {
	if f.ConversationID == "" || f.Seq <= 0 || f.Emoji == "" {
		sendErr(c, protocol.CodeBadFrame, "reaction requires conversation_id, seq, emoji")
		return
	}
	if !h.authorize(ctx, c, f.ConversationID) {
		return
	}
	octx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	var err error
	out := protocol.Frame{ConversationID: f.ConversationID, Seq: f.Seq, UserID: c.UserID(), Emoji: f.Emoji}
	if add {
		err = h.store.AddReaction(octx, f.ConversationID, f.Seq, c.UserID(), f.Emoji)
		out.Type = protocol.TypeReactionAdded
	} else {
		err = h.store.RemoveReaction(octx, f.ConversationID, f.Seq, c.UserID(), f.Emoji)
		out.Type = protocol.TypeReactionRemoved
	}
	if err != nil {
		sendErr(c, protocol.CodeInternal, "could not update reaction")
		return
	}
	_ = h.fan.Publish(octx, f.ConversationID, out)
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
	if err := h.fan.Publish(octx, f.ConversationID, protocol.Frame{
		Type:           protocol.TypeTyping,
		ConversationID: f.ConversationID,
		UserID:         c.UserID(),
		State:          state,
	}); err != nil {
		log.Printf("publish typing to %s: %v", f.ConversationID, err)
	}
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
