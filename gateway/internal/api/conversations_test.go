package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// doAuth issues a request carrying a bearer access token.
func doAuth(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

type convEnvelope struct {
	Data conversationView `json:"data"`
}

func createChannel(t *testing.T, h http.Handler, token, name string) string {
	t.Helper()
	rr := doAuth(t, h, "POST", "/conversations", token, createConversationReq{Kind: "channel", Name: name})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create conversation code=%d body=%s", rr.Code, rr.Body.String())
	}
	var ce convEnvelope
	_ = json.Unmarshal(rr.Body.Bytes(), &ce)
	return ce.Data.ID
}

func TestCreateListAndMembershipGate(t *testing.T) {
	h := testServer().Routes()
	a := signup(t, h, "a@b.com")
	b := signup(t, h, "b@b.com")

	convID := createChannel(t, h, a.AccessToken, "general-2")

	// Owner sees it in their list.
	list := doAuth(t, h, "GET", "/conversations", a.AccessToken, nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list code=%d", list.Code)
	}
	// Owner can fetch detail; a non-member cannot.
	if got := doAuth(t, h, "GET", "/conversations/"+convID, a.AccessToken, nil).Code; got != http.StatusOK {
		t.Fatalf("owner get detail code=%d", got)
	}
	if got := doAuth(t, h, "GET", "/conversations/"+convID, b.AccessToken, nil).Code; got != http.StatusForbidden {
		t.Fatalf("non-member get detail code=%d, want 403", got)
	}
}

func TestDM_Idempotent(t *testing.T) {
	h := testServer().Routes()
	a := signup(t, h, "a@b.com")
	b := signup(t, h, "b@b.com")

	first := doAuth(t, h, "POST", "/dms", a.AccessToken, createDMReq{UserID: b.User.ID})
	if first.Code != http.StatusOK {
		t.Fatalf("create dm code=%d body=%s", first.Code, first.Body.String())
	}
	var fe convEnvelope
	_ = json.Unmarshal(first.Body.Bytes(), &fe)

	// Same pair, opposite direction → same conversation.
	second := doAuth(t, h, "POST", "/dms", b.AccessToken, createDMReq{UserID: a.User.ID})
	var se convEnvelope
	_ = json.Unmarshal(second.Body.Bytes(), &se)
	if fe.Data.ID == "" || fe.Data.ID != se.Data.ID {
		t.Fatalf("DM not idempotent: %q vs %q", fe.Data.ID, se.Data.ID)
	}
}

func TestAddMember_RequiresManageRole(t *testing.T) {
	h := testServer().Routes()
	a := signup(t, h, "a@b.com") // owner
	b := signup(t, h, "b@b.com") // to be added
	c := signup(t, h, "c@b.com") // outsider

	convID := createChannel(t, h, a.AccessToken, "team")

	// Outsider cannot add members.
	if got := doAuth(t, h, "POST", "/conversations/"+convID+"/members", c.AccessToken,
		addMemberReq{UserID: b.User.ID}).Code; got != http.StatusForbidden {
		t.Fatalf("outsider add member code=%d, want 403", got)
	}
	// Owner can.
	if got := doAuth(t, h, "POST", "/conversations/"+convID+"/members", a.AccessToken,
		addMemberReq{UserID: b.User.ID}).Code; got != http.StatusOK {
		t.Fatalf("owner add member code=%d", got)
	}
	// Now b is a member and can read detail.
	if got := doAuth(t, h, "GET", "/conversations/"+convID, b.AccessToken, nil).Code; got != http.StatusOK {
		t.Fatalf("added member get detail code=%d", got)
	}
}

func TestHistoryAndReadGates(t *testing.T) {
	h := testServer().Routes()
	a := signup(t, h, "a@b.com")
	b := signup(t, h, "b@b.com")
	convID := createChannel(t, h, a.AccessToken, "hist")

	if got := doAuth(t, h, "GET", "/conversations/"+convID+"/messages", b.AccessToken, nil).Code; got != http.StatusForbidden {
		t.Fatalf("non-member history=%d, want 403", got)
	}
	if got := doAuth(t, h, "GET", "/conversations/"+convID+"/messages", a.AccessToken, nil).Code; got != http.StatusOK {
		t.Fatalf("member history=%d, want 200", got)
	}
	if got := doAuth(t, h, "POST", "/conversations/"+convID+"/read", a.AccessToken, map[string]any{}).Code; got != http.StatusBadRequest {
		t.Fatalf("read without seq=%d, want 400", got)
	}
	if got := doAuth(t, h, "POST", "/conversations/"+convID+"/read", a.AccessToken, readReq{Seq: 5}).Code; got != http.StatusOK {
		t.Fatalf("read with seq=%d, want 200", got)
	}
}

func TestBlockPreventsDM(t *testing.T) {
	h := testServer().Routes()
	a := signup(t, h, "a@b.com")
	b := signup(t, h, "b@b.com")

	if got := doAuth(t, h, "POST", "/blocks/"+b.User.ID, a.AccessToken, nil).Code; got != http.StatusOK {
		t.Fatalf("block code=%d", got)
	}
	if got := doAuth(t, h, "POST", "/dms", a.AccessToken, createDMReq{UserID: b.User.ID}).Code; got != http.StatusForbidden {
		t.Fatalf("DM after block code=%d, want 403", got)
	}
	// Block is bidirectional: b also cannot DM a.
	if got := doAuth(t, h, "POST", "/dms", b.AccessToken, createDMReq{UserID: a.User.ID}).Code; got != http.StatusForbidden {
		t.Fatalf("reverse DM code=%d, want 403", got)
	}
	doAuth(t, h, "DELETE", "/blocks/"+b.User.ID, a.AccessToken, nil)
	if got := doAuth(t, h, "POST", "/dms", a.AccessToken, createDMReq{UserID: b.User.ID}).Code; got != http.StatusOK {
		t.Fatalf("DM after unblock code=%d, want 200", got)
	}
}

func TestLeaveRemovesMembership(t *testing.T) {
	h := testServer().Routes()
	a := signup(t, h, "a@b.com")
	convID := createChannel(t, h, a.AccessToken, "leavable")

	if got := doAuth(t, h, "POST", "/conversations/"+convID+"/leave", a.AccessToken, nil).Code; got != http.StatusOK {
		t.Fatalf("leave code=%d", got)
	}
	if got := doAuth(t, h, "GET", "/conversations/"+convID, a.AccessToken, nil).Code; got != http.StatusForbidden {
		t.Fatalf("after leave get detail code=%d, want 403", got)
	}
}
