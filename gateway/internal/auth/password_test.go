package auth

import (
	"errors"
	"testing"
)

func TestHashPassword_RoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := VerifyPassword("correct horse battery staple", hash); err != nil {
		t.Fatalf("verify correct password: %v", err)
	}
	if err := VerifyPassword("wrong password", hash); !errors.Is(err, ErrPasswordMismatch) {
		t.Fatalf("verify wrong password = %v, want ErrPasswordMismatch", err)
	}
}

func TestHashPassword_DistinctSalts(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Fatal("two hashes of the same password are identical — salt not applied")
	}
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	for _, bad := range []string{"", "plain", "$argon2id$v=19$bad", "$bcrypt$x$y$z$w"} {
		if err := VerifyPassword("x", bad); !errors.Is(err, ErrInvalidHash) {
			t.Fatalf("VerifyPassword(%q) = %v, want ErrInvalidHash", bad, err)
		}
	}
}

func TestRefreshToken_HashDeterministicAndUnique(t *testing.T) {
	tok1, hash1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if tok1 == "" || hash1 == "" {
		t.Fatal("empty token or hash")
	}
	if HashRefreshToken(tok1) != hash1 {
		t.Fatal("HashRefreshToken not deterministic with GenerateRefreshToken")
	}
	tok2, hash2, _ := GenerateRefreshToken()
	if tok1 == tok2 || hash1 == hash2 {
		t.Fatal("two generated refresh tokens collide")
	}
}
