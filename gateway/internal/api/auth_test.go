package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Surge77/relay/gateway/internal/auth"
	"github.com/Surge77/relay/gateway/internal/model"
	"github.com/Surge77/relay/gateway/internal/store"
)

// fakeStore is an in-memory DataStore for handler tests — no Postgres needed.
type fakeStore struct {
	mu      sync.Mutex
	byEmail map[string]model.User
	byID    map[string]model.User
	tokens  map[string]model.RefreshToken
	convs   map[string]model.Conversation
	members map[string]map[string]string // conversationID -> userID -> role
	dms     map[string]string            // dmKey -> conversationID
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		byEmail: map[string]model.User{},
		byID:    map[string]model.User{},
		tokens:  map[string]model.RefreshToken{},
		convs:   map[string]model.Conversation{},
		members: map[string]map[string]string{},
		dms:     map[string]string{},
	}
}

func (f *fakeStore) CreateUser(_ context.Context, u model.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byEmail[u.Email]; ok {
		return &pgconn.PgError{Code: "23505"} // unique violation
	}
	f.byEmail[u.Email] = u
	f.byID[u.ID] = u
	return nil
}

func (f *fakeStore) UserByEmail(_ context.Context, email string) (model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return model.User{}, store.ErrNotFound
}

func (f *fakeStore) UserByID(_ context.Context, id string) (model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return model.User{}, store.ErrNotFound
}

func (f *fakeStore) InsertRefreshToken(_ context.Context, userID, hash string, exp time.Time, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[hash] = model.RefreshToken{UserID: userID, TokenHash: hash, ExpiresAt: exp}
	return nil
}

func (f *fakeStore) RefreshTokenByHash(_ context.Context, hash string) (model.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if rt, ok := f.tokens[hash]; ok {
		return rt, nil
	}
	return model.RefreshToken{}, store.ErrNotFound
}

func (f *fakeStore) RevokeRefreshToken(_ context.Context, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if rt, ok := f.tokens[hash]; ok {
		now := time.Now()
		rt.RevokedAt = &now
		f.tokens[hash] = rt
	}
	return nil
}

func (f *fakeStore) AddMember(_ context.Context, conv, user string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.members[conv] == nil {
		f.members[conv] = map[string]string{}
	}
	if _, ok := f.members[conv][user]; !ok {
		f.members[conv][user] = "member"
	}
	return nil
}

func (f *fakeStore) CreateConversation(_ context.Context, c model.Conversation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.convs[c.ID] = c
	f.members[c.ID] = map[string]string{c.CreatedBy: "owner"}
	return nil
}

func (f *fakeStore) GetOrCreateDM(_ context.Context, a, b string) (model.Conversation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := a + ":" + b
	if a > b {
		key = b + ":" + a
	}
	if id, ok := f.dms[key]; ok {
		return f.convs[id], nil
	}
	id := "dm_" + key
	c := model.Conversation{ID: id, Kind: "dm", CreatedBy: a}
	f.convs[id] = c
	f.dms[key] = id
	f.members[id] = map[string]string{a: "member", b: "member"}
	return c, nil
}

func (f *fakeStore) ListConversationsFor(_ context.Context, userID string) ([]model.ConversationSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.ConversationSummary
	for id, mm := range f.members {
		if _, ok := mm[userID]; ok {
			out = append(out, model.ConversationSummary{Conversation: f.convs[id]})
		}
	}
	return out, nil
}

func (f *fakeStore) ConversationDetail(_ context.Context, id string) (model.Conversation, []model.Member, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.convs[id]
	if !ok {
		return model.Conversation{}, nil, store.ErrNotFound
	}
	var ms []model.Member
	for u, role := range f.members[id] {
		ms = append(ms, model.Member{UserID: u, Role: role})
	}
	return c, ms, nil
}

func (f *fakeStore) MemberRole(_ context.Context, conv, user string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if mm, ok := f.members[conv]; ok {
		if role, ok := mm[user]; ok {
			return role, nil
		}
	}
	return "", store.ErrNotFound
}

func (f *fakeStore) RemoveMember(_ context.Context, conv, user string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if mm, ok := f.members[conv]; ok {
		delete(mm, user)
	}
	return nil
}

func (f *fakeStore) RenameConversation(_ context.Context, conv, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.convs[conv]; ok {
		c.Name = name
		f.convs[conv] = c
	}
	return nil
}

func (f *fakeStore) HistoryBefore(_ context.Context, _ string, _ int64, _ int) ([]model.Message, error) {
	return nil, nil
}

func (f *fakeStore) UnreadCount(_ context.Context, _, _ string) (int64, error) { return 0, nil }

func (f *fakeStore) SetLastRead(_ context.Context, _, _ string, _ int64) error { return nil }

type sessionEnvelope struct {
	Success bool        `json:"success"`
	Data    sessionResp `json:"data"`
}

func testServer() *Server {
	return NewServer(newFakeStore(), []byte("test-secret-32-bytes-or-whatever"), []string{"http://localhost:3000"}, nil)
}

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func signup(t *testing.T, h http.Handler, email string) sessionResp {
	t.Helper()
	rr := do(t, h, "POST", "/auth/signup", signupReq{Email: email, Password: "password123", DisplayName: "Tester"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("signup code=%d body=%s", rr.Code, rr.Body.String())
	}
	var se sessionEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &se); err != nil {
		t.Fatalf("decode signup: %v", err)
	}
	return se.Data
}

func TestSignup_IssuesValidSessionAndRejectsDuplicate(t *testing.T) {
	s := testServer()
	h := s.Routes()
	sess := signup(t, h, "a@b.com")
	if sess.AccessToken == "" || sess.RefreshToken == "" {
		t.Fatal("missing tokens")
	}
	uid, err := auth.Verify(s.secret, sess.AccessToken)
	if err != nil || uid != sess.User.ID {
		t.Fatalf("access token does not verify to new user: uid=%q err=%v", uid, err)
	}
	dup := do(t, h, "POST", "/auth/signup", signupReq{Email: "a@b.com", Password: "password123", DisplayName: "Other"})
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate email code=%d, want 409", dup.Code)
	}
}

func TestSignup_Validation(t *testing.T) {
	h := testServer().Routes()
	cases := []signupReq{
		{Email: "not-an-email", Password: "password123", DisplayName: "X"},
		{Email: "a@b.com", Password: "short", DisplayName: "X"},
		{Email: "a@b.com", Password: "password123", DisplayName: ""},
	}
	for _, c := range cases {
		rr := do(t, h, "POST", "/auth/signup", c)
		if rr.Code != http.StatusUnprocessableEntity {
			t.Fatalf("signup(%+v) code=%d, want 422", c, rr.Code)
		}
	}
}

func TestLogin_CorrectAndWrong(t *testing.T) {
	h := testServer().Routes()
	signup(t, h, "a@b.com")

	ok := do(t, h, "POST", "/auth/login", loginReq{Email: "a@b.com", Password: "password123"})
	if ok.Code != http.StatusOK {
		t.Fatalf("correct login code=%d", ok.Code)
	}
	wrong := do(t, h, "POST", "/auth/login", loginReq{Email: "a@b.com", Password: "wrongpassword"})
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password code=%d, want 401", wrong.Code)
	}
	unknown := do(t, h, "POST", "/auth/login", loginReq{Email: "nobody@b.com", Password: "password123"})
	if unknown.Code != http.StatusUnauthorized {
		t.Fatalf("unknown email code=%d, want 401", unknown.Code)
	}
}

func TestRefresh_RotatesAndSingleUse(t *testing.T) {
	h := testServer().Routes()
	sess := signup(t, h, "a@b.com")

	r1 := do(t, h, "POST", "/auth/refresh", refreshReq{RefreshToken: sess.RefreshToken})
	if r1.Code != http.StatusOK {
		t.Fatalf("refresh code=%d body=%s", r1.Code, r1.Body.String())
	}
	var se sessionEnvelope
	_ = json.Unmarshal(r1.Body.Bytes(), &se)
	if se.Data.RefreshToken == sess.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	// The presented (now revoked) token must not work again.
	reuse := do(t, h, "POST", "/auth/refresh", refreshReq{RefreshToken: sess.RefreshToken})
	if reuse.Code != http.StatusUnauthorized {
		t.Fatalf("reused old refresh code=%d, want 401", reuse.Code)
	}
	// The rotated token works.
	fresh := do(t, h, "POST", "/auth/refresh", refreshReq{RefreshToken: se.Data.RefreshToken})
	if fresh.Code != http.StatusOK {
		t.Fatalf("rotated refresh code=%d", fresh.Code)
	}
}

func TestLogout_RevokesRefresh(t *testing.T) {
	h := testServer().Routes()
	sess := signup(t, h, "a@b.com")

	lo := do(t, h, "POST", "/auth/logout", refreshReq{RefreshToken: sess.RefreshToken})
	if lo.Code != http.StatusOK {
		t.Fatalf("logout code=%d", lo.Code)
	}
	after := do(t, h, "POST", "/auth/refresh", refreshReq{RefreshToken: sess.RefreshToken})
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout code=%d, want 401", after.Code)
	}
}

func TestMe_RequiresValidToken(t *testing.T) {
	s := testServer()
	h := s.Routes()
	sess := signup(t, h, "a@b.com")

	noAuth := httptest.NewRecorder()
	h.ServeHTTP(noAuth, httptest.NewRequest("GET", "/auth/me", nil))
	if noAuth.Code != http.StatusUnauthorized {
		t.Fatalf("me without token code=%d, want 401", noAuth.Code)
	}

	req := httptest.NewRequest("GET", "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	withAuth := httptest.NewRecorder()
	h.ServeHTTP(withAuth, req)
	if withAuth.Code != http.StatusOK {
		t.Fatalf("me with token code=%d body=%s", withAuth.Code, withAuth.Body.String())
	}
}
