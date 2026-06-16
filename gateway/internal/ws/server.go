package ws

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"

	"github.com/Surge77/relay/gateway/internal/auth"
)

// Server upgrades HTTP requests to WebSockets, authenticates the handshake, and
// runs the read/write pumps for each connection.
type Server struct {
	secret         []byte
	handler        Handler
	sendBuf        int
	ratePerSec     int
	maxMessageByte int64
	originPatterns []string
}

// NewServer builds the WebSocket HTTP handler.
func NewServer(secret []byte, h Handler, sendBuf, ratePerSec, maxMessageBytes int, allowedOrigins []string) *Server {
	return &Server{
		secret:         secret,
		handler:        h,
		sendBuf:        sendBuf,
		ratePerSec:     ratePerSec,
		maxMessageByte: int64(maxMessageBytes),
		originPatterns: originHosts(allowedOrigins),
	}
}

// ServeHTTP handles the upgrade. The JWT is validated BEFORE the upgrade so an
// unauthenticated client never establishes a socket.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	userID, err := auth.Verify(s.secret, token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	wsc, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: s.originPatterns,
	})
	if err != nil {
		log.Printf("ws upgrade failed: %v", err) // Accept already wrote the response
		return
	}
	wsc.SetReadLimit(s.maxMessageByte)

	c := newConn(wsc, userID, s.sendBuf, s.ratePerSec)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	s.handler.OnConnect(c)

	// Sequenced teardown (not defer-ordered) to avoid racing a live Enqueue/Write
	// against Close: stop the writer, reject further enqueues, wait for the writer
	// to drain/exit, deregister, then close the socket.
	writeDone := make(chan struct{})
	go func() {
		c.writePump(ctx)
		close(writeDone)
	}()
	c.readPump(ctx, s.handler) // blocks until the connection ends

	cancel()
	close(c.closed)
	<-writeDone
	s.handler.OnDisconnect(c)
	wsc.Close(websocket.StatusNormalClosure, "")
}

// originHosts converts configured origin URLs (e.g. "http://localhost:3000")
// into host patterns ("localhost:3000") that coder/websocket matches against the
// request's Origin header.
func originHosts(origins []string) []string {
	out := make([]string, 0, len(origins))
	for _, o := range origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			out = append(out, u.Host)
			continue
		}
		out = append(out, strings.TrimSpace(o))
	}
	return out
}
