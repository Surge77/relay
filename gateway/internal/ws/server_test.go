package ws_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Surge77/relay/gateway/internal/auth"
	"github.com/Surge77/relay/gateway/internal/devinfra"
	"github.com/Surge77/relay/gateway/internal/hub"
	"github.com/Surge77/relay/gateway/internal/presence"
	"github.com/Surge77/relay/gateway/internal/protocol"
	"github.com/Surge77/relay/gateway/internal/registry"
	"github.com/Surge77/relay/gateway/internal/ws"
)

var secret = []byte("integration-secret")

func startServer(t *testing.T) string {
	t.Helper()
	reg := registry.New()
	store := devinfra.NewStore()
	store.AddMember("general", "alice")
	lf := devinfra.NewLocalFanout(nil)
	h := hub.New(reg, devinfra.NewSequencer(), store, lf, store, presence.Noop{})
	lf.SetDeliver(h.DeliverLocal)

	srv := ws.NewServer(secret, h, 64, 100, 16384, []string{"http://localhost:3000"})
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return "ws" + strings.TrimPrefix(ts.URL, "http")
}

func dial(t *testing.T, base, user string) *websocket.Conn {
	t.Helper()
	token, err := auth.Issue(secret, user)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, base+"/ws?token="+token, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { c.Close(websocket.StatusNormalClosure, "") })
	return c
}

func writeFrame(t *testing.T, c *websocket.Conn, f protocol.Frame) {
	t.Helper()
	data, _ := json.Marshal(f)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readFrame(t *testing.T, c *websocket.Conn) protocol.Frame {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var f protocol.Frame
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return f
}

func TestHandshake_RejectsBadToken(t *testing.T) {
	base := startServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, base+"/ws?token=garbage", nil)
	if err == nil {
		t.Fatal("expected dial to fail with bad token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestSend_AckOverTheWire(t *testing.T) {
	base := startServer(t)
	c := dial(t, base, "alice")
	writeFrame(t, c, protocol.Frame{Type: protocol.TypeSubscribe, ConversationID: "general"})
	if f := readFrame(t, c); f.Type != protocol.TypeCaughtUp {
		t.Fatalf("first frame = %s, want caughtup", f.Type)
	}
	writeFrame(t, c, protocol.Frame{Type: protocol.TypeSend, ConversationID: "general", ClientMsgID: "m1", Body: "hello"})

	// Sender is a local subscriber, so it receives both the fanned-out message
	// and the ack (order between them is not guaranteed).
	var gotMsg, gotAck bool
	for i := 0; i < 2; i++ {
		f := readFrame(t, c)
		switch f.Type {
		case protocol.TypeMessage:
			if f.Body == "hello" && f.Seq == 1 {
				gotMsg = true
			}
		case protocol.TypeAck:
			if f.ClientMsgID == "m1" && f.Seq == 1 {
				gotAck = true
			}
		}
	}
	if !gotMsg || !gotAck {
		t.Fatalf("gotMsg=%v gotAck=%v, want both", gotMsg, gotAck)
	}
}
