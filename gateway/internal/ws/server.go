package ws

import (
	"context"
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
		return // Accept already wrote the error response
	}
	wsc.SetReadLimit(s.maxMessageByte)

	c := newConn(wsc, userID, s.sendBuf, s.ratePerSec)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	s.handler.OnConnect(c)
	defer s.handler.OnDisconnect(c)
	defer close(c.closed)

	go c.writePump(ctx)
	c.readPump(ctx, s.handler) // blocks until the connection ends

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
