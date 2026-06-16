# Relay

Horizontally-scalable real-time messaging backend.

**Target: 50k concurrent WebSocket connections / 5 nodes · fan-out p99 < 200 ms · 10k msgs/sec · zero message loss on single-node kill.**

> Headline numbers are recorded in [`docs/benchmarks.md`](docs/benchmarks.md) from real
> load runs — see *Reproduce the numbers* below. Until a run is recorded, treat them as targets.

## Why

The hard part is keeping one message correct and fast when sender and receiver sit on
different machines: cross-node fan-out, per-conversation ordering, at-least-once delivery,
gap-free reconnect, and presence at scale. The chat UI is the easy 10%.

Two correctness invariants drive the design:

1. **Durable before live.** A message is appended to the durable log *before* it is
   published for live fan-out. Anything ever delivered live is therefore always replayable on
   reconnect — a delivered message can never vanish from history.
2. **One sequencer per conversation.** Every message gets a strictly-increasing, gap-free
   `seq`. It is the sole ordering and dedupe key. The sequence counter recovers from Postgres
   (`MAX(seq)`) on cold start, so a Redis restart can never regress it.

## Stack

Go gateway · Redis (pub/sub + streams) · Postgres · Next.js client. No Docker, no external
accounts, no API keys — two local services (Postgres + a Redis-compatible server).

## Architecture

See [`docs/architecture.md`](docs/architecture.md). In short: any gateway node terminates any
client socket; a message published on one node reaches members hosted on any other node via
Redis pub/sub; Redis Streams provide durable persistence + offline queues; Postgres holds
metadata and partitioned message history.

## Run locally (Windows, no Docker)

Prerequisites: Go 1.26+, Node.js LTS, Postgres 17, a Redis-compatible server
([Memurai](https://www.memurai.com) on Windows, or `redis-server` in WSL2).

```powershell
# 1. Start infrastructure (Postgres service + Memurai). Run elevated once:
./scripts/dev-up.ps1

# 2. Configure
copy .env.example .env   # then fill JWT_SECRET + POSTGRES_URL password

# 3. Migrate the database
migrate -path migrations -database $env:POSTGRES_URL up

# 4. Run a gateway node
cd gateway
go run ./cmd/gateway

# 5. Talk to it with the CLI client (separate terminals)
go run ./cmd/relayctl -user alice -conv general -secret <your JWT_SECRET>
go run ./cmd/relayctl -user bob   -conv general -secret <your JWT_SECRET>
```

For a zero-dependency single-node demo (no Postgres/Redis), set `RELAY_DEV_INMEM=1`.

## Test

```powershell
cd gateway
go build ./... && go vet ./...
go test ./...                 # unit + in-memory integration
go test -tags=integration ./... # live PG + Redis required
```

CI (Linux) additionally runs `go test -race`. The local Windows gcc cannot build the race
detector, so run race checks in CI.

## Reproduce the numbers

```powershell
k6 run load/relay-load.js   # ramps connections; prints p50/p99
```

Record the output in `docs/benchmarks.md` with the exact command and hardware.

## Roadmap / out of scope (v1)

Multi-region, end-to-end encryption, media attachments, message edit/delete/threads, mobile
push, channels > 1k members, and Kafka/Scylla scale-out are documented migration paths, not
built in v1. See the full plan in [`relay-implementation-plan.md`](relay-implementation-plan.md).
