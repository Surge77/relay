package api

import (
	"net/http"
	"net/mail"
	"time"

	"github.com/google/uuid"

	"github.com/Surge77/relay/gateway/internal/auth"
	"github.com/Surge77/relay/gateway/internal/model"
	"github.com/Surge77/relay/gateway/internal/store"
)

const (
	refreshTTL     = 30 * 24 * time.Hour
	minPasswordLen = 8
	maxPasswordLen = 200
	refreshCookie  = "relay_refresh"
)

type signupReq struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

type sessionResp struct {
	User         userView `json:"user"`
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
}

type userView struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	StatusText  string `json:"status_text,omitempty"`
}

func viewOf(u model.User) userView {
	return userView{
		ID: u.ID, Email: u.Email, DisplayName: u.DisplayName,
		AvatarURL: u.AvatarURL, StatusText: u.StatusText,
	}
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req signupReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body")
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_EMAIL", "a valid email is required")
		return
	}
	if len(req.Password) < minPasswordLen || len(req.Password) > maxPasswordLen {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_PASSWORD", "password must be 8–200 characters")
		return
	}
	if req.DisplayName == "" {
		writeErr(w, http.StatusUnprocessableEntity, "INVALID_NAME", "display name is required")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not create account")
		return
	}
	u := model.User{ID: uuid.NewString(), Email: req.Email, DisplayName: req.DisplayName, PasswordHash: hash}
	if err := s.store.CreateUser(r.Context(), u); err != nil {
		if store.IsDuplicate(err) {
			writeErr(w, http.StatusConflict, "EMAIL_TAKEN", "that email is already registered")
			return
		}
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not create account")
		return
	}
	s.respondSession(w, r, u, http.StatusCreated)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body")
		return
	}
	u, err := s.store.UserByEmail(r.Context(), req.Email)
	// Same response whether the user is missing or the password is wrong, so the
	// endpoint does not reveal which emails are registered.
	if err != nil || u.PasswordHash == "" || auth.VerifyPassword(req.Password, u.PasswordHash) != nil {
		writeErr(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "incorrect email or password")
		return
	}
	s.respondSession(w, r, u, http.StatusOK)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	raw := s.refreshTokenFrom(r)
	if raw == "" {
		writeErr(w, http.StatusUnauthorized, "NO_REFRESH", "missing refresh token")
		return
	}
	rec, err := s.store.RefreshTokenByHash(r.Context(), auth.HashRefreshToken(raw))
	if err != nil || rec.RevokedAt != nil || time.Now().After(rec.ExpiresAt) {
		writeErr(w, http.StatusUnauthorized, "INVALID_REFRESH", "refresh token invalid or expired")
		return
	}
	// Rotate: revoke the presented token before minting a replacement, so a
	// stolen refresh token is single-use.
	_ = s.store.RevokeRefreshToken(r.Context(), rec.TokenHash)
	u, err := s.store.UserByID(r.Context(), rec.UserID)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "INVALID_REFRESH", "account not found")
		return
	}
	s.respondSession(w, r, u, http.StatusOK)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if raw := s.refreshTokenFrom(r); raw != "" {
		_ = s.store.RevokeRefreshToken(r.Context(), auth.HashRefreshToken(raw))
	}
	clearRefreshCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.UserByID(r.Context(), userIDFrom(r.Context()))
	if err != nil {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	writeJSON(w, http.StatusOK, viewOf(u))
}

// respondSession issues an access token + a fresh refresh token, persists the
// refresh hash, sets the httpOnly cookie, and returns both in the body (the body
// copy serves non-browser clients like relayctl and tests).
func (s *Server) respondSession(w http.ResponseWriter, r *http.Request, u model.User, status int) {
	access, err := auth.Issue(s.secret, u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not issue token")
		return
	}
	raw, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not issue token")
		return
	}
	if err := s.store.InsertRefreshToken(r.Context(), u.ID, hash, time.Now().Add(refreshTTL), r.UserAgent()); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", "could not persist session")
		return
	}
	setRefreshCookie(w, raw)
	writeJSON(w, status, sessionResp{User: viewOf(u), AccessToken: access, RefreshToken: raw})
}

// refreshTokenFrom reads the refresh token from the httpOnly cookie (browser) or
// the JSON body (CLI/tests).
func (s *Server) refreshTokenFrom(r *http.Request) string {
	if c, err := r.Cookie(refreshCookie); err == nil && c.Value != "" {
		return c.Value
	}
	var req refreshReq
	if err := decodeJSON(r, &req); err == nil {
		return req.RefreshToken
	}
	return ""
}

func setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookie, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: int(refreshTTL / time.Second),
	})
}

func clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: refreshCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
}
