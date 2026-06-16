// Command relayctl is a throwaway CLI WebSocket client for exercising the
// gateway before any browser UI exists. It mints a dev token, connects,
// subscribes to a conversation, prints inbound frames, and sends whatever you
// type on stdin.
//
// Usage:
//
//	relayctl -user alice -conv general -addr ws://localhost:8080/ws \
//	         -secret $JWT_SECRET
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/Surge77/relay/gateway/internal/auth"
	"github.com/Surge77/relay/gateway/internal/protocol"
)

func main() {
	user := flag.String("user", "alice", "user id to log in as")
	conv := flag.String("conv", "general", "conversation id to subscribe to")
	addr := flag.String("addr", "ws://localhost:8080/ws", "gateway WebSocket address")
	secret := flag.String("secret", os.Getenv("JWT_SECRET"), "JWT signing secret")
	lastSeq := flag.Int64("since", 0, "last acked seq for catch-up")
	flag.Parse()

	if *secret == "" {
		log.Fatal("JWT secret required via -secret or JWT_SECRET env")
	}
	token, err := auth.Issue([]byte(*secret), *user)
	if err != nil {
		log.Fatalf("issue token: %v", err)
	}

	ctx := context.Background()
	c, _, err := websocket.Dial(ctx, *addr+"?token="+token, nil)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	writeFrame(ctx, c, protocol.Frame{
		Type:           protocol.TypeSubscribe,
		ConversationID: *conv,
		LastAckedSeq:   *lastSeq,
	})

	go readLoop(ctx, c)

	fmt.Printf("connected as %s to %s. type messages and press enter:\n", *user, *conv)
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		body := scan.Text()
		if body == "" {
			continue
		}
		writeFrame(ctx, c, protocol.Frame{
			Type:           protocol.TypeSend,
			ClientMsgID:    uuid.NewString(),
			ConversationID: *conv,
			Body:           body,
		})
	}
}

func readLoop(ctx context.Context, c *websocket.Conn) {
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			log.Printf("read closed: %v", err)
			return
		}
		var f protocol.Frame
		if err := json.Unmarshal(data, &f); err != nil {
			continue
		}
		switch f.Type {
		case protocol.TypeMessage:
			fmt.Printf("  [%d] %s: %s\n", f.Seq, f.SenderID, f.Body)
		case protocol.TypeAck:
			fmt.Printf("  ack %s -> seq %d\n", f.ClientMsgID, f.Seq)
		case protocol.TypeCaughtUp:
			fmt.Printf("  caught up at seq %d\n", f.Seq)
		case protocol.TypeError:
			fmt.Printf("  error %s: %s\n", f.Code, f.Message)
		default:
			fmt.Printf("  %s %+v\n", f.Type, f)
		}
	}
}

func writeFrame(ctx context.Context, c *websocket.Conn, f protocol.Frame) {
	data, _ := json.Marshal(f)
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := c.Write(wctx, websocket.MessageText, data); err != nil {
		log.Printf("write: %v", err)
	}
}
