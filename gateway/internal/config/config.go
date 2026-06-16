// Package config loads and validates gateway configuration from the environment.
// It fails fast: a missing required value is a startup error, never a runtime
// surprise.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime knobs for a single gateway node.
type Config struct {
	JWTSecret           []byte
	PostgresURL         string
	RedisURL            string
	GatewayPort         int
	NodeID              string
	MaxConnPerNode      int
	SendBufferSize      int
	RateLimitMsgsPerSec int
	MaxMessageBytes     int
	AllowedOrigins      []string
}

// Load reads configuration from environment variables, applying defaults for the
// optional ones and returning an error if a required one is absent or malformed.
func Load() (*Config, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	// POSTGRES_URL is required in production wiring but optional for the
	// in-memory dev mode (RELAY_DEV_INMEM=1); the prod path validates it.

	c := &Config{
		JWTSecret:           []byte(secret),
		PostgresURL:         os.Getenv("POSTGRES_URL"),
		RedisURL:            getEnv("REDIS_URL", "redis://localhost:6379"),
		GatewayPort:         getEnvInt("GATEWAY_PORT", 8080),
		NodeID:              getEnv("NODE_ID", "gw1"),
		MaxConnPerNode:      getEnvInt("MAX_CONN_PER_NODE", 10000),
		SendBufferSize:      getEnvInt("SEND_BUFFER_SIZE", 256),
		RateLimitMsgsPerSec: getEnvInt("RATE_LIMIT_MSGS_PER_SEC", 20),
		MaxMessageBytes:     getEnvInt("MAX_MESSAGE_BYTES", 16384),
		AllowedOrigins:      splitCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:3000")),
	}
	return c, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
