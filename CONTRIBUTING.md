# Contributing to Relay

## Before you start

- Read the architecture in `docs/architecture.md`.
- Open an issue describing the change before large PRs.

## Dev setup

See the README → "Run locally". Requires Go 1.26+, Node LTS, Postgres 17, and a
Redis-compatible server (Memurai or WSL `redis-server`).

## Workflow

1. Branch from `main`: `feature/<desc>` or `fix/<desc>`.
2. Write tests first (TDD). No implementation without a test.
3. Run the gates locally before pushing:
   - Gateway: `cd gateway && go build ./... && go vet ./... && go test ./...`
   - Web: `cd web && npm run build && npm run lint && npm test`
4. Use Conventional Commits (below).
5. Open a PR using the template. CI must be green.

## Commit format

`<type>(<scope>): <description>` — types: feat, fix, docs, style, refactor, test, chore, perf, ci.

## Code rules

- The gateway holds no global state; each node owns only its own socket registry.
- The sequencer is the only writer of `seq`.
- Durable persist happens **before** live fan-out (see README invariants).
- Every external call has a timeout (default 5s).
- File size limit 300 lines; split by responsibility.
- No secrets in code or logs; never log message bodies.
