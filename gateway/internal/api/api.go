// Package api is the REST control plane: authentication today, and (in later
// phases) conversations, profiles, search, and uploads. It shares the Postgres
// store and auth packages with the realtime gateway but runs as its own service,
// so the stateless control plane scales independently of the socket fleet.
package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/Surge77/relay/gateway/internal/model"
)

// maxBodyBytes caps a request body to keep JSON decoding bounded.
const maxBodyBytes = 1 << 20

// DataStore is the persistence surface the control plane needs. Defined here
// (where it is consumed) so handlers can be unit-tested with a fake; *store.Store
// satisfies it.
type DataStore interface {
	CreateUser(ctx context.Context, u model.User) error
	UserByEmail(ctx context.Context, email string) (model.User, error)
	UserByID(ctx context.Context, id string) (model.User, error)
	InsertRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time, userAgent string) error
	RefreshTokenByHash(ctx context.Context, tokenHash string) (model.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	AddMember(ctx context.Context, conversationID, userID string) error

	CreateConversation(ctx context.Context, c model.Conversation) error
	GetOrCreateDM(ctx context.Context, userA, userB string) (model.Conversation, error)
	ListConversationsFor(ctx context.Context, userID string) ([]model.ConversationSummary, error)
	ConversationDetail(ctx context.Context, conversationID string) (model.Conversation, []model.Member, error)
	MemberRole(ctx context.Context, conversationID, userID string) (string, error)
	RemoveMember(ctx context.Context, conversationID, userID string) error
	RenameConversation(ctx context.Context, conversationID, name string) error
}

// Server holds the control-plane dependencies and builds the HTTP router.
type Server struct {
	store   DataStore
	secret  []byte
	origins map[string]bool
}

// NewServer wires the control plane. allowedOrigins are echoed back for CORS so
// the browser can send credentials (the refresh cookie).
func NewServer(st DataStore, secret []byte, allowedOrigins []string) *Server {
	origins := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		origins[o] = true
	}
	return &Server{store: st, secret: secret, origins: origins}
}

// Routes returns the HTTP handler for the control plane.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/signup", s.handleSignup)
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("POST /auth/refresh", s.handleRefresh)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.Handle("GET /auth/me", s.requireAuth(http.HandlerFunc(s.handleMe)))

	mux.Handle("POST /conversations", s.requireAuth(http.HandlerFunc(s.handleCreateConversation)))
	mux.Handle("GET /conversations", s.requireAuth(http.HandlerFunc(s.handleListConversations)))
	mux.Handle("GET /conversations/{id}", s.requireAuth(http.HandlerFunc(s.handleGetConversation)))
	mux.Handle("PATCH /conversations/{id}", s.requireAuth(http.HandlerFunc(s.handleRenameConversation)))
	mux.Handle("POST /conversations/{id}/members", s.requireAuth(http.HandlerFunc(s.handleAddMember)))
	mux.Handle("DELETE /conversations/{id}/members/{userId}", s.requireAuth(http.HandlerFunc(s.handleRemoveMember)))
	mux.Handle("POST /conversations/{id}/leave", s.requireAuth(http.HandlerFunc(s.handleLeave)))
	mux.Handle("POST /dms", s.requireAuth(http.HandlerFunc(s.handleCreateDM)))

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	return s.withCORS(mux)
}

// envelope is the shared API response shape: success carries data, failure
// carries a user-safe error code + message. Internal detail is never leaked.
type envelope struct {
	Success   bool      `json:"success"`
	Data      any       `json:"data,omitempty"`
	Error     *apiError `json:"error,omitempty"`
	Timestamp string    `json:"timestamp"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Success: true, Data: data, Timestamp: nowRFC3339()})
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{
		Success: false, Error: &apiError{Code: code, Message: msg}, Timestamp: nowRFC3339(),
	})
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
