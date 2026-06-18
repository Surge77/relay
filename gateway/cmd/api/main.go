// Command api is the Relay REST control plane: authentication today, and
// conversations / profiles / search / uploads in later phases. It is stateless
// and shares Postgres with the realtime gateway, so it scales independently of
// the WebSocket fleet.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Surge77/relay/gateway/internal/api"
	"github.com/Surge77/relay/gateway/internal/config"
	"github.com/Surge77/relay/gateway/internal/events"
	"github.com/Surge77/relay/gateway/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	if cfg.PostgresURL == "" {
		log.Error("POSTGRES_URL is required for the api service")
		os.Exit(1)
	}

	st, err := store.New(context.Background(), cfg.PostgresURL)
	if err != nil {
		log.Error("postgres", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	// Redis is used only to publish control-plane events onto the gateway
	// fan-out; the API holds no socket state itself.
	var publisher api.EventPublisher
	if opts, err := redis.ParseURL(cfg.RedisURL); err == nil {
		publisher = events.NewPublisher(redis.NewClient(opts))
	} else {
		log.Warn("redis url invalid; realtime events disabled", "err", err)
	}

	port := getEnvInt("API_PORT", 9000)
	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           api.NewServer(st, cfg.JWTSecret, cfg.AllowedOrigins, publisher).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("relay api listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	log.Info("relay api stopped")
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
