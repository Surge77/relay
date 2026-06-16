// Package auth handles the JWT presented on the WebSocket handshake. Tokens are
// short-lived and validated before any frame is processed; a failed validation
// closes the connection.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenTTL is the maximum lifetime of a handshake token. Kept short so a leaked
// token has a small blast radius; clients refresh via the REST control plane.
const TokenTTL = 15 * time.Minute

// Claims is the JWT payload. Subject (sub) is the user id.
type Claims struct {
	jwt.RegisteredClaims
}

// ErrInvalidToken is returned for any token that fails signature, expiry, or
// structural validation. The specific reason is intentionally not surfaced to
// the client.
var ErrInvalidToken = errors.New("invalid token")

// Verify checks the token's signature and expiry and returns the user id (sub).
func Verify(secret []byte, tokenStr string) (string, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	var claims Claims
	_, err := parser.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		return "", ErrInvalidToken
	}
	if claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}

// Issue mints a token for the given user id. Used by tests and the dev "log in
// as" helper; in production the REST control plane owns issuance.
func Issue(secret []byte, userID string) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}
