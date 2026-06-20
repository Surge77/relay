package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Surge77/relay/gateway/internal/auth"
	"github.com/Surge77/relay/gateway/internal/model"
)

const testSecret = "test-secret-32-bytes-or-whatever"

// searchServer builds a control plane over a seeded fake store, marking the
// given users online, and returns the handler plus a bearer token for caller.
func searchServer(t *testing.T, caller string, online map[string]bool, users ...model.User) (http.Handler, string) {
	t.Helper()
	store := newFakeStore()
	for _, u := range users {
		if err := store.CreateUser(t.Context(), u); err != nil {
			t.Fatalf("seed user %s: %v", u.ID, err)
		}
	}
	s := NewServer(store, []byte(testSecret), []string{"http://localhost:3000"}, nil, nil, fakePresence{online: online})
	tok, err := auth.Issue(s.secret, caller)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return s.Routes(), tok
}

func getAuthed(t *testing.T, h http.Handler, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestSearchUsers_RejectsShortQuery(t *testing.T) {
	h, tok := searchServer(t, "me", nil)
	for _, q := range []string{"", "a", "%20"} { // "%20" decodes to " " -> trimmed empty
		rr := getAuthed(t, h, "/users/search?q="+q, tok)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("q=%q code=%d, want 422", q, rr.Code)
		}
	}
}

func TestSearchUsers_MatchesNameAndEmail_ExcludesSelf(t *testing.T) {
	h, tok := searchServer(t, "me",
		map[string]bool{"u-alice": true},
		model.User{ID: "me", Email: "me@x.com", DisplayName: "Me Myself"},
		model.User{ID: "u-alice", Email: "alice@x.com", DisplayName: "Alice Anderson"},
		model.User{ID: "u-bob", Email: "bob@x.com", DisplayName: "Bob Brown"},
	)

	// Match by name.
	var byName []profileView
	decodeData(t, getAuthed(t, h, "/users/search?q=alice", tok), &byName)
	if len(byName) != 1 || byName[0].ID != "u-alice" {
		t.Fatalf("name search = %+v, want [u-alice]", byName)
	}
	if !byName[0].Online {
		t.Fatal("u-alice should be reported online")
	}

	// Match by email substring; self (me@x.com) must never appear.
	var byEmail []profileView
	decodeData(t, getAuthed(t, h, "/users/search?q=x.com", tok), &byEmail)
	for _, u := range byEmail {
		if u.ID == "me" {
			t.Fatal("search must exclude the caller")
		}
	}
	if len(byEmail) != 2 {
		t.Fatalf("email search returned %d users, want 2 (alice+bob)", len(byEmail))
	}
}

func TestSearchUsers_NeverLeaksEmail(t *testing.T) {
	h, tok := searchServer(t, "me", nil,
		model.User{ID: "u-alice", Email: "alice@secret.com", DisplayName: "Alice"})
	rr := getAuthed(t, h, "/users/search?q=alice", tok)
	if strings.Contains(rr.Body.String(), "secret.com") {
		t.Fatalf("response leaked email: %s", rr.Body.String())
	}
}

func TestSearchUsers_ExcludesBlocked(t *testing.T) {
	h, tok := searchServer(t, "me", nil,
		model.User{ID: "u-alice", Email: "alice@x.com", DisplayName: "Alice"})
	// me blocks alice (via the block endpoint path) -> alice disappears from search.
	blockReq := httptest.NewRequest("POST", "/blocks/u-alice", nil)
	blockReq.Header.Set("Authorization", "Bearer "+tok)
	rrBlock := httptest.NewRecorder()
	h.ServeHTTP(rrBlock, blockReq)
	if rrBlock.Code != http.StatusOK {
		t.Fatalf("block code=%d", rrBlock.Code)
	}

	var res []profileView
	decodeData(t, getAuthed(t, h, "/users/search?q=alice", tok), &res)
	if len(res) != 0 {
		t.Fatalf("blocked user still in results: %+v", res)
	}
}

func decodeData(t *testing.T, rr *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var env struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if err := json.Unmarshal(env.Data, dst); err != nil {
		t.Fatalf("decode data: %v", err)
	}
}
