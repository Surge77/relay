// Command devredis runs an in-memory, RESP-compatible Redis server (miniredis)
// on a real TCP port, so several gateway nodes can share one broker for local
// multi-node runs with no Redis/Memurai install and no Docker.
//
// NOT for production: all state is in memory and lost on exit. There is no AOF,
// so restarting THIS process drops the durable stream — broker-restart message
// durability is the one guarantee that still needs a real Redis.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:6379", "address to listen on")
	flag.Parse()

	m := miniredis.NewMiniRedis()
	if err := m.StartAddr(*addr); err != nil {
		log.Fatalf("devredis: start on %s: %v", *addr, err)
	}
	defer m.Close()
	log.Printf("devredis (miniredis) listening on %s", m.Addr())

	// miniredis never advances its own clock, so TTL keys (presence markers)
	// would otherwise live forever. Step the clock forward in real time so
	// presence self-heal expires dead markers like a real server would.
	stopTick := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-stopTick:
				return
			case <-t.C:
				m.FastForward(time.Second)
			}
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	close(stopTick)
	log.Printf("devredis stopped")
}
