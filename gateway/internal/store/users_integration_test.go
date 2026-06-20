//go:build integration

// Requires a live Postgres with migrations applied (POSTGRES_URL).
// Run with: go test -tags=integration ./internal/store/
package store

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// seedUser inserts a throwaway account and returns its id. A unique tag is
// woven into the display name + email so searches stay isolated from the dev
// seed users (alice/bob/carol) and from other tests.
func seedUser(t *testing.T, s *Store, tag string) string {
	t.Helper()
	id := "user-" + uuid.NewString()
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO users (id, display_name, email, password_hash) VALUES ($1,$2,$3,'x')`,
		id, "Name "+tag, tag+"@example.test")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func TestSearchUsers(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	tag := strings.ReplaceAll(uuid.NewString()[:8], "-", "")

	alice := seedUser(t, s, "alice"+tag)
	_ = seedUser(t, s, "bob"+tag)
	me := seedUser(t, s, "me"+tag)

	// Matches by display-name substring, case-insensitive.
	got, err := s.SearchUsers(ctx, strings.ToUpper("alice"+tag), me, 30)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 || got[0].ID != alice {
		t.Fatalf("name search = %+v, want [%s]", got, alice)
	}

	// Matches by email substring and returns both alice+bob; never the caller.
	got, err = s.SearchUsers(ctx, tag, me, 30)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("shared-tag search = %d users, want 2", len(got))
	}
	for _, u := range got {
		if u.ID == me {
			t.Fatal("search must exclude the caller")
		}
	}

	// Limit is honored.
	got, err = s.SearchUsers(ctx, tag, me, 1)
	if err != nil || len(got) != 1 {
		t.Fatalf("limited search = %d,%v; want 1,nil", len(got), err)
	}
}

func TestSearchUsers_ExcludesBlocked(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	tag := strings.ReplaceAll(uuid.NewString()[:8], "-", "")

	me := seedUser(t, s, "me"+tag)
	other := seedUser(t, s, "other"+tag)

	if got, _ := s.SearchUsers(ctx, "other"+tag, me, 30); len(got) != 1 {
		t.Fatalf("pre-block search = %d, want 1", len(got))
	}
	if err := s.AddBlock(ctx, other, me); err != nil { // other blocked me — still hidden
		t.Fatalf("add block: %v", err)
	}
	if got, _ := s.SearchUsers(ctx, "other"+tag, me, 30); len(got) != 0 {
		t.Fatalf("post-block search = %d, want 0 (block is bidirectional)", len(got))
	}
}

func TestTouchLastSeen(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	id := seedUser(t, s, "seen"+strings.ReplaceAll(uuid.NewString()[:8], "-", ""))

	before, err := s.UserByID(ctx, id)
	if err != nil {
		t.Fatalf("user by id: %v", err)
	}
	if before.LastSeenAt != nil {
		t.Fatal("last_seen_at should be nil before any connection")
	}
	if err := s.TouchLastSeen(ctx, id); err != nil {
		t.Fatalf("touch: %v", err)
	}
	after, err := s.UserByID(ctx, id)
	if err != nil {
		t.Fatalf("user by id: %v", err)
	}
	if after.LastSeenAt == nil {
		t.Fatal("last_seen_at should be set after TouchLastSeen")
	}
}
