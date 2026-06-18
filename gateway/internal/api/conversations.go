package api

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/Surge77/relay/gateway/internal/model"
)

type createConversationReq struct {
	Kind    string   `json:"kind"` // channel | group
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

type createDMReq struct {
	UserID string `json:"user_id"`
}

type addMemberReq struct {
	UserID string `json:"user_id"`
}

type renameReq struct {
	Name string `json:"name"`
}

type conversationView struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name,omitempty"`
	CreatedBy string `json:"created_by,omitempty"`
}

func convView(c model.Conversation) conversationView {
	return conversationView{ID: c.ID, Kind: c.Kind, Name: c.Name, CreatedBy: c.CreatedBy}
}

func (s *Server) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	actor := userIDFrom(r.Context())
	var req createConversationReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body")
		return
	}
	if req.Kind != "channel" && req.Kind != "group" {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_KIND", "kind must be channel or group")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_NAME", "name is required")
		return
	}
	c := model.Conversation{ID: uuid.NewString(), Kind: req.Kind, Name: req.Name, CreatedBy: actor}
	if err := s.store.CreateConversation(r.Context(), c); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not create conversation")
		return
	}
	for _, m := range req.Members {
		if m != "" && m != actor {
			_ = s.store.AddMember(r.Context(), c.ID, m)
		}
	}
	writeJSON(w, http.StatusCreated, convView(c))
}

func (s *Server) handleCreateDM(w http.ResponseWriter, r *http.Request) {
	actor := userIDFrom(r.Context())
	var req createDMReq
	if err := decodeJSON(r, &req); err != nil || req.UserID == "" {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "user_id is required")
		return
	}
	if req.UserID == actor {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_TARGET", "cannot DM yourself")
		return
	}
	if _, err := s.store.UserByID(r.Context(), req.UserID); err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	c, err := s.store.GetOrCreateDM(r.Context(), actor, req.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not open direct message")
		return
	}
	writeJSON(w, http.StatusOK, convView(c))
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListConversationsFor(r.Context(), userIDFrom(r.Context()))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not list conversations")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !s.isMember(r, convID) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "not a member of this conversation")
		return
	}
	c, members, err := s.store.ConversationDetail(r.Context(), convID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": convView(c), "members": members})
}

func (s *Server) handleAddMember(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !s.canManage(r, convID) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "only an owner or admin can add members")
		return
	}
	var req addMemberReq
	if err := decodeJSON(r, &req); err != nil || req.UserID == "" {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "user_id is required")
		return
	}
	if err := s.store.AddMember(r.Context(), convID, req.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not add member")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	target := r.PathValue("userId")
	// A member may remove themselves; otherwise the actor must be owner/admin.
	if target != userIDFrom(r.Context()) && !s.canManage(r, convID) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "cannot remove that member")
		return
	}
	if err := s.store.RemoveMember(r.Context(), convID, target); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not remove member")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLeave(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RemoveMember(r.Context(), r.PathValue("id"), userIDFrom(r.Context())); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not leave conversation")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRenameConversation(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !s.canManage(r, convID) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "only an owner or admin can rename")
		return
	}
	var req renameReq
	if err := decodeJSON(r, &req); err != nil || req.Name == "" {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_NAME", "name is required")
		return
	}
	if err := s.store.RenameConversation(r.Context(), convID, req.Name); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not rename")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// isMember reports whether the request's user belongs to the conversation.
func (s *Server) isMember(r *http.Request, convID string) bool {
	_, err := s.store.MemberRole(r.Context(), convID, userIDFrom(r.Context()))
	return err == nil
}

// canManage reports whether the request's user is an owner or admin (any lookup
// failure is treated as not-permitted).
func (s *Server) canManage(r *http.Request, convID string) bool {
	role, err := s.store.MemberRole(r.Context(), convID, userIDFrom(r.Context()))
	return err == nil && (role == "owner" || role == "admin")
}
