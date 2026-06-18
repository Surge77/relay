package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// refreshTokenBytes is the entropy of an opaque refresh token (256 bits).
const refreshTokenBytes = 32

// GenerateRefreshToken returns a new opaque refresh token to hand to the client
// and its storage hash. The raw token is never persisted — only the hash is, so
// a database leak cannot be replayed as a session.
func GenerateRefreshToken() (token, hash string, err error) {
	b := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("refresh token: %w", err)
	}
	token = hex.EncodeToString(b)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken returns the storage hash of a raw refresh token. Refresh
// tokens are high-entropy random values, so a fast hash (SHA-256) is sufficient;
// there is nothing to brute-force, unlike a password.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
