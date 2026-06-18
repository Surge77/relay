package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Surge77/relay/gateway/internal/model"
)

// ErrNotFound is returned when a lookup matches no row, so handlers can map it to
// a 401/404 instead of leaking a database error to the client.
var ErrNotFound = errors.New("not found")

// IsDuplicate reports whether err is a Postgres unique-constraint violation
// (SQLSTATE 23505) — e.g. signing up with an already-registered email.
func IsDuplicate(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

const userColumns = `id, COALESCE(email,''), display_name, COALESCE(password_hash,''),
	COALESCE(avatar_url,''), COALESCE(status_text,'')`

// CreateUser inserts a new account. Empty email/password are stored as NULL so a
// credential-less dev user does not collide on the unique email index.
func (s *Store) CreateUser(ctx context.Context, u model.User) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, display_name, email, password_hash)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''))`,
		u.ID, u.DisplayName, u.Email, u.PasswordHash)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// UserByEmail looks up an account by email for login. Returns ErrNotFound if no
// such user exists.
func (s *Store) UserByEmail(ctx context.Context, email string) (model.User, error) {
	return s.scanUser(ctx, `SELECT `+userColumns+` FROM users WHERE email=$1`, email)
}

// UserByID looks up an account by id.
func (s *Store) UserByID(ctx context.Context, id string) (model.User, error) {
	return s.scanUser(ctx, `SELECT `+userColumns+` FROM users WHERE id=$1`, id)
}

func (s *Store) scanUser(ctx context.Context, q, arg string) (model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx, q, arg).Scan(
		&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.AvatarURL, &u.StatusText)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}

// UpdateProfile updates a user's editable profile fields. Display name is only
// changed when a non-empty value is supplied; status and avatar are set as given.
func (s *Store) UpdateProfile(ctx context.Context, userID, displayName, statusText, avatarURL string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET display_name=COALESCE(NULLIF($2,''), display_name),
		    status_text=$3, avatar_url=NULLIF($4,''), updated_at=now() WHERE id=$1`,
		userID, displayName, statusText, avatarURL)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return nil
}

// AddBlock records that blocker has blocked blocked (idempotent).
func (s *Store) AddBlock(ctx context.Context, blockerID, blockedID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("add block: %w", err)
	}
	return nil
}

// RemoveBlock removes a block.
func (s *Store) RemoveBlock(ctx context.Context, blockerID, blockedID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM blocks WHERE blocker_id=$1 AND blocked_id=$2`, blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("remove block: %w", err)
	}
	return nil
}

// IsBlocked reports whether either user has blocked the other.
func (s *Store) IsBlocked(ctx context.Context, a, b string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM blocks
		   WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))`,
		a, b).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is blocked: %w", err)
	}
	return exists, nil
}

// SetMute sets (or clears, when until is nil) a member's mute expiry.
func (s *Store) SetMute(ctx context.Context, conversationID, userID string, until *time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE memberships SET muted_until=$3 WHERE conversation_id=$1 AND user_id=$2`,
		conversationID, userID, until)
	if err != nil {
		return fmt.Errorf("set mute: %w", err)
	}
	return nil
}

// InsertRefreshToken records a newly issued refresh token (by hash).
func (s *Store) InsertRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time, userAgent string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent)
		 VALUES ($1, $2, $3, NULLIF($4,''))`,
		userID, tokenHash, expiresAt, userAgent)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// RefreshTokenByHash fetches a refresh-token record for validation. Returns
// ErrNotFound when the token is unknown.
func (s *Store) RefreshTokenByHash(ctx context.Context, tokenHash string) (model.RefreshToken, error) {
	var rt model.RefreshToken
	err := s.pool.QueryRow(ctx,
		`SELECT user_id, token_hash, expires_at, revoked_at
		   FROM refresh_tokens WHERE token_hash=$1`, tokenHash).Scan(
		&rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.RefreshToken{}, ErrNotFound
	}
	if err != nil {
		return model.RefreshToken{}, fmt.Errorf("refresh token by hash: %w", err)
	}
	return rt, nil
}

// RevokeRefreshToken marks a token revoked (idempotent). Used on rotation and
// logout.
func (s *Store) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at=now()
		  WHERE token_hash=$1 AND revoked_at IS NULL`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}
