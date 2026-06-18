package api

import (
	"net/http"
	"strconv"

	"github.com/Surge77/relay/gateway/internal/model"
)

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 200
)

type messageView struct {
	ConversationID string `json:"conversation_id"`
	Seq            int64  `json:"seq"`
	SenderID       string `json:"sender_id"`
	ClientMsgID    string `json:"client_msg_id,omitempty"`
	Body           string `json:"body"`
	TS             int64  `json:"ts"`
}

func msgView(m model.Message) messageView {
	return messageView{
		ConversationID: m.ConversationID, Seq: m.Seq, SenderID: m.SenderID,
		ClientMsgID: m.ClientMsgID, Body: m.Body, TS: m.TS,
	}
}

type readReq struct {
	Seq int64 `json:"seq"`
}

// handleHistory returns a page of older messages (scrollback) for a conversation
// the caller belongs to. ?before=<seq> pages backward; ?limit caps the page.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !s.isMember(r, convID) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "not a member of this conversation")
		return
	}
	before := parseInt(r.URL.Query().Get("before"), 0)
	limit := parseInt(r.URL.Query().Get("limit"), defaultHistoryLimit)
	if limit <= 0 || limit > maxHistoryLimit {
		limit = defaultHistoryLimit
	}
	msgs, err := s.store.HistoryBefore(r.Context(), convID, before, int(limit))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not load history")
		return
	}
	out := make([]messageView, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, msgView(m))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUnread(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !s.isMember(r, convID) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "not a member of this conversation")
		return
	}
	n, err := s.store.UnreadCount(r.Context(), convID, userIDFrom(r.Context()))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not compute unread")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"unread": n})
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !s.isMember(r, convID) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "not a member of this conversation")
		return
	}
	var req readReq
	if err := decodeJSON(r, &req); err != nil || req.Seq <= 0 {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "seq is required")
		return
	}
	if err := s.store.SetLastRead(r.Context(), convID, userIDFrom(r.Context()), req.Seq); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not record read")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func parseInt(s string, def int64) int64 {
	if s == "" {
		return def
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	return def
}
