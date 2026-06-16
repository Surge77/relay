package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("test-secret-not-for-production")

func TestVerify_RoundTrip(t *testing.T) {
	token, err := Issue(testSecret, "user-42")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	sub, err := Verify(testSecret, token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if sub != "user-42" {
		t.Fatalf("sub = %q, want user-42", sub)
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	token, _ := Issue(testSecret, "user-1")
	if _, err := Verify([]byte("other-secret"), token); err != ErrInvalidToken {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
}

func TestVerify_Expired(t *testing.T) {
	claims := Claims{RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "u",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testSecret)
	if _, err := Verify(testSecret, tok); err != ErrInvalidToken {
		t.Fatalf("err = %v, want ErrInvalidToken for expired token", err)
	}
}

func TestVerify_NoExpiryRejected(t *testing.T) {
	claims := Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "u"}}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testSecret)
	if _, err := Verify(testSecret, tok); err != ErrInvalidToken {
		t.Fatalf("err = %v, want ErrInvalidToken when exp missing", err)
	}
}

func TestVerify_EmptySubject(t *testing.T) {
	if _, err := Verify(testSecret, ""); err != ErrInvalidToken {
		t.Fatalf("err = %v, want ErrInvalidToken for empty token", err)
	}
}
