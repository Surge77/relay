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

## How to reproduce

1. Start 5 gateway nodes (`NODE_ID`/`GATEWAY_PORT` per node) against one Postgres + Redis.
2. Point a load balancer (or k6 multiple targets) at the fleet.
3. `k6 run load/relay-load.js` — ramps connections per the stages in that script.
4. Record p50/p99 from the k6 summary into the table above.
