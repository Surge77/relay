# Changelog

All notable changes are documented here. Format: [Keep a Changelog](https://keepachangelog.com);
versioning: [SemVer](https://semver.org).

## [Unreleased]

### Added
- Phase 0: repository scaffold, hygiene files, CI skeleton, `.env.example`, dev scripts.
- Phase 1: single-node Go gateway — JWT handshake, node-local socket registry, conversation
  subscribe + send/echo path, per-connection rate limiting, frame-size cap, throwaway CLI
  client (`relayctl`). In-memory infrastructure for dependency-free local runs.
- Phase 2: Postgres metadata + hash-partitioned message history (golang-migrate), `pgx` store
  with idempotent persist, and the Redis-backed sequencer with Postgres-seed recovery
  (strictly-increasing, gap-free; property-tested).
- Phase 3: Redis pub/sub cross-node fan-out; multi-node wiring; durable-before-live ordering.
- Phase 4: reconnect + catch-up by `last_acked_seq` with seq-dedupe; node-kill chaos test
  proving zero loss / zero duplicate.
- Phase 5: presence (Redis TTL keys + heartbeat, self-healing), typing indicators, and read
  receipts persisted to `memberships.last_read_seq`.
- Phase 6: durable Redis Streams pipeline — every message appended to a durable stream and
  drained by a consumer group into Postgres; at-least-once with restart re-delivery (tested).
- Phase 7: Next.js + TypeScript + Tailwind client — `useChatSocket` (auth, capped backoff
  reconnect, resume-by-cursor), seq-ordered `MessageList`, optimistic `Composer`,
  `ConnectionBadge`, Zustand store, and a dev "log in as" token endpoint.
