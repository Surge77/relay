// Command gateway runs a single Relay WebSocket gateway node.
//
// With RELAY_DEV_INMEM=1 it uses in-memory infrastructure (no Redis/Postgres) —
// the Phase 1 single-node demo. Otherwise it wires the Redis + Postgres backed
// stack (added in later phases).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Surge77/relay/gateway/internal/config"
	"github.com/Surge77/relay/gateway/internal/devinfra"
	"github.com/Surge77/relay/gateway/internal/fanout"
	"github.com/Surge77/relay/gateway/internal/hub"
	"github.com/Surge77/relay/gateway/internal/model"
	"github.com/Surge77/relay/gateway/internal/presence"
	"github.com/Surge77/relay/gateway/internal/push"
	"github.com/Surge77/relay/gateway/internal/registry"
	"github.com/Surge77/relay/gateway/internal/sequencer"
	"github.com/Surge77/relay/gateway/internal/store"
	"github.com/Surge77/relay/gateway/internal/stream"
	"github.com/Surge77/relay/gateway/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	reg := registry.New()
	h, cleanup, err := buildHub(appCtx, cfg, reg)
	if err != nil {
		log.Fatalf("init: %v", err)
	}
	defer cleanup()

	srv := ws.NewServer(cfg.JWTSecret, h, cfg.SendBufferSize, cfg.RateLimitMsgsPerSec, cfg.MaxMessageBytes, cfg.AllowedOrigins)

	mux := http.NewServeMux()
	mux.Handle("/ws", srv)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

	httpSrv := &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.GatewayPort),
		Handler: mux,
	}

	go func() {
		log.Printf("relay gateway node=%s listening on :%d (dev_inmem=%v)", cfg.NodeID, cfg.GatewayPort, devMode())
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	log.Printf("relay gateway node=%s stopped", cfg.NodeID)
}

func devMode() bool { return os.Getenv("RELAY_DEV_INMEM") == "1" }

// buildHub assembles the hub with either in-memory (dev) or Redis+Postgres
// (production) infrastructure. It returns a cleanup func to release resources.
func buildHub(ctx context.Context, cfg *config.Config, reg *registry.Registry) (*hub.Hub, func(), error) {
	if devMode() {
		return buildDevHub(reg)
	}
	return buildProdHub(ctx, cfg, reg)
}

// buildDevHub wires the dependency-free in-memory stack for local demos.
func buildDevHub(reg *registry.Registry) (*hub.Hub, func(), error) {
	mem := devinfra.NewStore()
	for _, u := range []string{"alice", "bob", "carol"} {
		mem.AddMember("general", u)
	}
	lf := devinfra.NewLocalFanout(nil)
	h := hub.New(reg, devinfra.NewSequencer(), mem, lf, mem, presence.NewMemory())
	lf.SetDeliver(h.DeliverLocal)
	return h, func() {}, nil
}

// buildProdHub wires Postgres (metadata + history + durable persist), the
// Redis-backed sequencer, and Redis pub/sub fan-out.
func buildProdHub(ctx context.Context, cfg *config.Config, reg *registry.Registry) (*hub.Hub, func(), error) {
	if cfg.PostgresURL == "" {
		log.Fatal("POSTGRES_URL is required (or set RELAY_DEV_INMEM=1 for the in-memory demo)")
	}
	pg, err := store.New(ctx, cfg.PostgresURL)
	if err != nil {
		return nil, nil, err
	}
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		pg.Close()
		return nil, nil, err
	}
	rdb := redis.NewClient(opts)

	seq := sequencer.NewRedis(rdb, pg)
	fan := fanout.NewRedis(ctx, rdb)
	pres := presence.NewRedis(rdb)

	// Durable-first pipeline: messages are appended to the Redis Stream (the
	// durable record) before live fan-out, and an in-process consumer group
	// drains the stream into Postgres history. Postgres persistence is idempotent,
	// so the at-least-once consumer can safely re-deliver after a restart.
	//
	// Notifications ride this async drain (off the message hot path): after each
	// message is persisted, offline members are pushed. A failing push never fails
	// persistence.
	pushSvc := push.NewService(pg, pres, push.LogNotifier{})
	durableLog := stream.NewLog(rdb)
	consumer := stream.NewConsumer(rdb, cfg.NodeID, func(sctx context.Context, m model.Message) error {
		if err := pg.Persist(sctx, m); err != nil {
			return err
		}
		pushSvc.NotifyOfflineMembers(sctx, m)
		return nil
	})
	go func() {
		if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("stream consumer stopped: %v", err)
		}
	}()

	h := hub.New(reg, seq, pg, fan, durableLog, pres)
	fan.SetDeliver(h.DeliverLocal)

	cleanup := func() {
		fan.Close()
		rdb.Close()
		pg.Close()
	}
	return h, cleanup, nil
}
