package api

import (
	"net/http"
	"time"
)

type profileView struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	StatusText  string `json:"status_text,omitempty"`
}

type updateProfileReq struct {
	DisplayName string `json:"display_name"`
	StatusText  string `json:"status_text"`
	AvatarURL   string `json:"avatar_url"`
}

type muteReq struct {
	Minutes int `json:"minutes"` // 0 clears the mute
}

// handleGetUser returns a user's public profile (no email).
func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.UserByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	writeJSON(w, http.StatusOK, profileView{ID: u.ID, DisplayName: u.DisplayName, AvatarURL: u.AvatarURL, StatusText: u.StatusText})
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req updateProfileReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body")
		return
	}
	uid := userIDFrom(r.Context())
	if err := s.store.UpdateProfile(r.Context(), uid, req.DisplayName, req.StatusText, req.AvatarURL); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not update profile")
		return
	}
	u, err := s.store.UserByID(r.Context(), uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not load profile")
		return
	}
	writeJSON(w, http.StatusOK, profileView{ID: u.ID, DisplayName: u.DisplayName, AvatarURL: u.AvatarURL, StatusText: u.StatusText})
}

func (s *Server) handleBlock(w http.ResponseWriter, r *http.Request) {
	if err := s.store.AddBlock(r.Context(), userIDFrom(r.Context()), r.PathValue("userId")); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not block user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUnblock(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RemoveBlock(r.Context(), userIDFrom(r.Context()), r.PathValue("userId")); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not unblock user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMute(w http.ResponseWriter, r *http.Request) {
	convID := r.PathValue("id")
	if !s.isMember(r, convID) {
		writeErr(w, http.StatusForbidden, "FORBIDDEN", "not a member of this conversation")
		return
	}
	var req muteReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body")
		return
	}
	var until *time.Time
	if req.Minutes > 0 {
		t := time.Now().Add(time.Duration(req.Minutes) * time.Minute)
		until = &t
	}
	if err := s.store.SetMute(r.Context(), convID, userIDFrom(r.Context()), until); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not update mute")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
