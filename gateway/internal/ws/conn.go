// Package ws is the WebSocket transport: it upgrades connections, authenticates
// the handshake, and runs the per-connection read/write pumps. All message
// routing is delegated to a Handler so the transport stays free of business
// logic.
package ws

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/Surge77/relay/gateway/internal/protocol"
	"github.com/Surge77/relay/gateway/internal/registry"
)

// pongWait is the longest a connection may go without sending any frame before
// the read pump treats it as dead. Clients send app-level pings to stay under it.
const pongWait = 90 * time.Second

// Handler receives connection lifecycle and frame events from the transport.
type Handler interface {
	OnConnect(c registry.Client)
	OnDisconnect(c registry.Client)
	OnFrame(ctx context.Context, c registry.Client, f protocol.Frame)
}

// Conn is one client WebSocket. It implements registry.Client.
type Conn struct {
	id      string
	userID  string
	ws      *websocket.Conn
	send    chan protocol.Frame
	limiter *rateLimiter
	closed  chan struct{}
}

func newConn(wsc *websocket.Conn, userID string, sendBuf, ratePerSec int) *Conn {
	return &Conn{
		id:      uuid.NewString(),
		userID:  userID,
		ws:      wsc,
		send:    make(chan protocol.Frame, sendBuf),
		limiter: newRateLimiter(ratePerSec),
		closed:  make(chan struct{}),
	}
}

func (c *Conn) ID() string     { return c.id }
func (c *Conn) UserID() string { return c.userID }

// Enqueue performs a non-blocking send. It returns false if the buffer is full
// or the connection is closing, so one slow client can never stall fan-out.
func (c *Conn) Enqueue(f protocol.Frame) bool {
	select {
	case <-c.closed:
		return false
	case c.send <- f:
		return true
	default:
		return false
	}
}

// writePump is the sole writer for the socket. It drains the send channel until
// the connection closes.
func (c *Conn) writePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closed:
			return
		case f := <-c.send:
			data, err := json.Marshal(f)
			if err != nil {
				continue
			}
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err = c.ws.Write(wctx, websocket.MessageText, data)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

// readPump is the sole reader. It enforces the frame-size limit (set by the
// server) and the per-connection rate limit, then dispatches each frame to the
// handler. It returns when the connection errors or closes.
func (c *Conn) readPump(ctx context.Context, h Handler) {
	for {
		rctx, cancel := context.WithTimeout(ctx, pongWait)
		_, data, err := c.ws.Read(rctx)
		cancel()
		if err != nil {
			return
		}
		if !c.limiter.allow() {
			c.Enqueue(protocol.Frame{Type: protocol.TypeError, Code: protocol.CodeRateLimited, Message: "slow down"})
			continue
		}
		var f protocol.Frame
		if err := json.Unmarshal(data, &f); err != nil {
			c.Enqueue(protocol.Frame{Type: protocol.TypeError, Code: protocol.CodeBadFrame, Message: "malformed frame"})
			continue
		}
		h.OnFrame(ctx, c, f)
	}
}
