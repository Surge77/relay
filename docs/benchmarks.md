# Benchmarks

> Every number here must come from a real run, with the exact command and hardware recorded.
> Until a row is filled from an actual run, it shows the **target**, not an achieved result.

## Environment template

| Field | Value |
|-------|-------|
| Date | _TBD_ |
| Machine (CPU / RAM) | _TBD_ |
| OS | _TBD_ |
| Go version | 1.26 |
| Gateway nodes | _TBD_ |
| Postgres / Redis | _TBD_ |

## Results

| Metric | Target | Achieved | Command |
|--------|--------|----------|---------|
| Concurrent connections (held 10 min) | 50,000 / 5 nodes | _TBD_ | `k6 run load/relay-load.js` |
| Echo p99 (single node) | < 20 ms | _TBD_ | `k6 run load/relay-load.js` |
| Cross-node fan-out p99 | < 200 ms | _TBD_ | `k6 run load/relay-load.js` |
| History read p99 | < 100 ms | _TBD_ | `k6 run load/relay-load.js` |
| Throughput | 10,000 msgs/sec | _TBD_ | `k6 run load/relay-load.js` |
| Node-kill message loss | 0 | _TBD_ | chaos test (`go test -tags=integration -run Chaos`) |

The fleet rows above stay `_TBD_` until run on a real multi-node deployment — a
single dev laptop cannot host 50k connections across 5 nodes. The section below
records what *was* measured locally, clearly scoped.

## Local single-node run (measured)

> Scope: one in-memory gateway node (`RELAY_DEV_INMEM=1`, no Redis/Postgres),
> k6 co-resident on the same machine (so it competes for CPU and inflates the
> latency tails — true server-side numbers are lower). Single conversation,
> all VUs as one member. This validates the hot path and handshake; it is **not**
> the 50k fleet number.

| Field | Value |
|-------|-------|
| Date | 2026-06-19 |
| Machine | Intel Core i5-12450H (12 logical), 15.7 GB RAM |
| OS | Windows 11 Home |
| Go version | 1.26.0 |
| k6 version | 2.0.0 |
| Gateway nodes | 1 (in-memory) |
| Peak concurrent connections | 300 |

| Metric | Measured | Notes |
|--------|----------|-------|
| Send→ack roundtrip | med 4 ms, p90 19 ms, p95 28 ms, max 61 ms | `relay_ack_ms` custom trend |
| WS handshake | med 1.7 ms, p95 19.9 ms | `ws_connecting` |
| Fan-out delivery throughput | 52,629 msgs/sec (6.05M delivered) | single node, ~252× fan-out into a 300-member room |
| Messages sent | 23,991 (209/sec) | source send rate |
| Failed connections | 0 / 300 | `checks_succeeded` 100% |

Command (single-node local, scaled stage targets via env overrides):

```
k6 run load/relay-load.js \
  -e WS_ADDR=ws://localhost:8080/ws -e TOKEN=<member JWT> -e CONV=general \
  -e WARM_VUS=100 -e MID_VUS=200 -e PEAK_VUS=300 \
  -e RAMP_DUR=15s -e MID_DUR=15s -e PEAK_DUR=15s -e HOLD_DUR=30s -e DOWN_DUR=10s
```

Gateway: `JWT_SECRET=<secret> RELAY_DEV_INMEM=1 go run ./cmd/gateway`.
The token's `sub` must be a seeded member of the conversation (`alice`/`bob`/
`carol` on `general` in dev) or sends are rejected and no acks return.

## How to reproduce

1. Start 5 gateway nodes (`NODE_ID`/`GATEWAY_PORT` per node) against one Postgres + Redis.
2. Point a load balancer (or k6 multiple targets) at the fleet.
3. `k6 run load/relay-load.js` — ramps connections per the stages in that script.
4. Record p50/p99 from the k6 summary into the table above.
