package api

import "net/http"

const (
	defaultSearchLimit = 30
	maxSearchLimit     = 100
)

// handleSearch runs a full-text search over messages in the caller's
// conversations. GET /search?q=<query>&limit=<n>.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_QUERY", "q is required")
		return
	}
	limit := parseInt(r.URL.Query().Get("limit"), defaultSearchLimit)
	if limit <= 0 || limit > maxSearchLimit {
		limit = defaultSearchLimit
	}
	msgs, err := s.store.SearchMessages(r.Context(), userIDFrom(r.Context()), q, int(limit))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "search failed")
		return
	}
	out := make([]messageView, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, msgView(m))
	}
	writeJSON(w, http.StatusOK, out)
}
