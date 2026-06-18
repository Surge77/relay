package api

import "net/http"

type pushSubscribeReq struct {
	Endpoint string `json:"endpoint"`
	P256dh   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

type pushUnsubscribeReq struct {
	Endpoint string `json:"endpoint"`
}

func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	var req pushSubscribeReq
	if err := decodeJSON(r, &req); err != nil || req.Endpoint == "" || req.P256dh == "" || req.Auth == "" {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "endpoint, p256dh and auth are required")
		return
	}
	if err := s.store.SavePushSubscription(r.Context(), userIDFrom(r.Context()), req.Endpoint, req.P256dh, req.Auth); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not save subscription")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var req pushUnsubscribeReq
	if err := decodeJSON(r, &req); err != nil || req.Endpoint == "" {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "endpoint is required")
		return
	}
	if err := s.store.DeletePushSubscription(r.Context(), userIDFrom(r.Context()), req.Endpoint); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not remove subscription")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
