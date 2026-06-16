# Relay — Distributed Real-Time Chat Backend
## Complete Implementation & Repository Plan

> A horizontally-scalable real-time messaging backend supporting 1:1 DMs and group channels, with presence, typing indicators, read receipts, durable history, and offline delivery — engineered so any gateway node can serve any user while messages stay correctly ordered and exactly-once-visible across the fleet.
>
> **Headline metric (the README number):** sustains 50k concurrent WebSocket connections across a 5-node gateway fleet, fan-out p99 < 200 ms same-region, 10k msgs/sec, zero message loss under single-node kill.

**Project name:** Relay
**Repo / folder:** `relay`
**Platform:** Windows (no Docker — native Postgres + Redis, optional WSL2)
**License:** MIT

---

## Table of Contents
1. [What This Project Is](#1-what-this-project-is)
2. [Tech Stack (Windows-native v1)](#2-tech-stack-windows-native-v1)
3. [No External Services / API Keys](#3-no-external-services--api-keys)
4. [Architecture](#4-architecture)
5. [Repository Structure](#5-repository-structure)
6. [Local Environment Setup (Windows, no Docker)](#6-local-environment-setup-windows-no-docker)
7. [Implementation Phases (build order with measurables)](#7-implementation-phases-build-order-with-measurables)
8. [UI Plan](#8-ui-plan)
9. [GitHub Repository — Full Setup](#9-github-repository--full-setup)
10. [Required Repo Files (with templates)](#10-required-repo-files-with-templates)
11. [Branching, Commit & Push Rules](#11-branching-commit--push-rules)
12. [CI/CD & Automation](#12-cicd--automation)
13. [Security Policy & Practices](#13-security-policy--practices)
14. [Maintenance Plan](#14-maintenance-plan)
15. [Testing Strategy](#15-testing-strategy)
16. [Acceptance Criteria (definition of done)](#16-acceptance-criteria-definition-of-done)
17. [Milestone Timeline](#17-milestone-timeline)
18. [Out of Scope (v1)](#18-out-of-scope-v1)

---

## 1. What This Project Is

The hard, resume-worthy part is **not** the chat UI. It is keeping a single message **correct and fast** when sender and receiver are connected to **different machines**. That requires:

- **Fan-out across nodes** — a message arriving on Gateway 1 must reach a recipient on Gateway 3.
- **Per-conversation ordering** — no consumer ever sees messages out of order.
- **Delivery guarantees** — at-least-once delivery + client-side dedupe; nothing lost when a node dies.
- **Reconnect / catch-up** — client reconnects to *any* node and resumes with no gaps, no duplicates.
- **Presence at scale** — online/offline/typing propagated quickly without flooding the fleet.

**Target audience for the design choices:** interviewers / system-design reviewers. The README, the architecture diagram, and the reproducible numbers are as much a deliverable as the code.

---

## 2. Tech Stack (Windows-native v1)

| Layer | Choice | Why |
|-------|--------|-----|
| Connection gateway | **Go** | goroutine-per-connection = cleanest 10k-sockets/node story |
| Stateless API / control plane | Go (or Java Spring Boot) | stateless behind LB; start single-language |
| Cross-node fan-out | **Redis Pub/Sub** | routes a message to nodes hosting online members |
| Durable async pipeline | **Redis Streams** | consumer groups + replay without Kafka's Windows pain |
| Metadata store | **PostgreSQL** | users, channels, memberships, read receipts, sequencer |
| Message history | **Postgres partitioned table** (v1) | partition by `conversation_id`, clustering on `seq` |
| Presence | **Redis** (TTL keys + heartbeat) | cheap, ephemeral |
| Client | **Next.js + TypeScript + Tailwind**, native `WebSocket` | matches existing brand; raw WS shows the protocol |
| Client state | **Zustand** (or `useReducer`) | status discriminant union, not boolean soup |
| Load testing | **k6** (or custom Go harness) | ramp to 50k conns, capture p50/p99 |

**Deliberately deferred (Windows-friendliness + scope):**
- **Kafka → Redis Streams.** Tradeoff to state in README: Redis Streams = simpler, single-node durability; Kafka = partitioned horizontal scale. Fine for a portfolio demo.
- **Cassandra/Scylla → partitioned Postgres.** Describe the migration path past X writes/sec; describing the migration *is* the senior signal — you do not have to run it.

> **Go vs Java gateway tradeoff:** Go gives the densest connection count and simplest concurrency — the strongest "real-time systems" signal and the cheapest path to 50k. Java/Netty fits a fintech-backend brand but fights the JVM thread/memory model to hit the same density. **Recommendation: Go gateway.** Don't split languages prematurely.

---

## 3. No External Services / API Keys

This is **pure-logic infra**. Everything runs locally. No paid APIs, no third-party accounts.

| Thing | External account? | Key? |
|-------|-------------------|------|
| Redis | No — local | No |
| Postgres | No — local | Local password you set |
| JWT signing secret | No — you generate it | Random string in `.env` |

**Optional, all v1-out-of-scope:**
- Mobile push → Firebase FCM (free key) — skipped.
- Public cloud scale demo → AWS/GCP — optional; minikube or one beefy box works.
- Public TLS cert → Let's Encrypt (free) — local dev uses self-signed.

**Net: zero external accounts, zero API keys, zero Docker, two local services.**

---

## 4. Architecture

```
            ┌─────────────┐
 clients ──▶│  WS LB      │ (any node works on reconnect)
            └──────┬──────┘
        ┌──────────┼──────────┐
        ▼          ▼          ▼
   ┌────────┐ ┌────────┐ ┌────────┐
   │Gateway1│ │Gateway2│ │Gateway3│   each holds only its own socket registry
   └───┬────┘ └───┬────┘ └───┬────┘
       └──────────┼──────────┘
                  ▼
          ┌───────────────┐   pub/sub channel per conversation
          │ Redis Pub/Sub │   routes msg to nodes hosting members
          └───────┬───────┘
                  ▼
          ┌───────────────┐
          │ Redis Streams │  durable: persist + offline queues + replay
          └───────┬───────┘
                  ▼
          ┌───────────────┐
          │   Postgres    │  metadata + partitioned message history
          └───────────────┘
   Redis (presence TTL + last-seen)
```

**Message lifecycle (send):**
1. Client → Gateway: `{client_msg_id, conversation_id, body}`.
2. Gateway validates membership + auth, calls sequencer → assigns `seq` (Redis `INCR conv:{id}:seq`, periodic Postgres checkpoint).
3. Gateway publishes to Redis channel `conv:{id}` **and** appends to Redis Stream `messages.persist`.
4. Every gateway subscribed to `conv:{id}` (hosting an online member) receives it, pushes to that socket.
5. Stream consumer writes to Postgres history + enqueues offline members.
6. Gateway acks `{client_msg_id → seq}` to sender.

**Reconnect / catch-up:** client sends `last_acked_seq` per conversation → gateway pulls the gap from history → replays → resumes live stream. Dedupe by `seq`.

---

## 5. Repository Structure

```
relay/
├── gateway/                 # Go WebSocket gateway (the core)
│   ├── cmd/
│   ├── internal/
│   │   ├── auth/            # JWT handshake
│   │   ├── registry/        # local socket registry
│   │   ├── fanout/          # Redis pub/sub
│   │   ├── sequencer/       # per-conversation seq
│   │   └── stream/          # Redis Streams persist/offline
│   └── go.mod
├── services/                # stateless REST control plane
├── web/                     # Next.js client
├── migrations/              # versioned SQL (golang-migrate)
├── load/                    # k6 / load harness
├── docs/
│   ├── architecture.md
│   ├── architecture.png     # the diagram
│   └── benchmarks.md        # the numbers + how to reproduce
├── scripts/                 # setup/dev helpers (PowerShell + bash)
├── .github/                 # see section 9
├── .env.example
├── .gitignore
├── README.md
├── LICENSE
├── SECURITY.md
├── CONTRIBUTING.md
├── CODE_OF_CONDUCT.md
├── CHANGELOG.md
└── relay-implementation-plan.md   # this plan (copy in repo)
```

> Keep this **separate** from the `firstApply` repo. Suggested path: `C:\Users\tdmne\Desktop\experiments\relay`.

---

## 6. Local Environment Setup (Windows, no Docker)

### Option A — WSL2 (recommended, not Docker)
WSL2 is a real Linux kernel built into Windows. Your Go/Node code runs on the Windows side and reaches services over `localhost`.

```powershell
wsl --install -d Ubuntu      # one time, then reboot
```
Inside Ubuntu:
```bash
sudo apt update
sudo apt install redis-server postgresql -y
sudo service redis-server start
sudo service postgresql start
```

### Option B — fully native Windows
| Service | How |
|---------|-----|
| Postgres | Official EDB installer; runs as a Windows service |
| Redis | Memurai (Redis-compatible, built for Windows) — free dev tier |

### Toolchain (Windows side, either option)
- Go (latest stable) — `winget install GoLang.Go`
- Node.js LTS — `winget install OpenJS.NodeJS.LTS`
- `golang-migrate` for migrations
- k6 — `winget install k6.k6`
- `git` + GitHub CLI (`gh`)

### `.env` (generated locally — never committed)
```
JWT_SECRET=<random-32-byte-hex>
POSTGRES_URL=postgres://relay:<password>@localhost:5432/relay
REDIS_URL=redis://localhost:6379
GATEWAY_PORT=8080
MAX_CONN_PER_NODE=10000
SEND_BUFFER_SIZE=256
RATE_LIMIT_MSGS_PER_SEC=20
```

---

## 7. Implementation Phases (build order with measurables)

Each phase ends with a number. **No phase is "done" without its measurable recorded in `docs/benchmarks.md`.** Build backend first; UI is the final 10%.

| Phase | Build | Measurable (must record) |
|-------|-------|--------------------------|
| 0 | Repo init, hygiene files, CI skeleton, `.env.example` | `go build` + `npm run build` green in CI |
| 1 | Single-node gateway, in-memory rooms, echo | 10k conns/node, p99 echo < 20 ms |
| 2 | Postgres metadata + sequencer + history | ordered history read p99 < 100 ms |
| 3 | Redis Pub/Sub fan-out → multi-node | cross-node fan-out p99 < 200 ms |
| 4 | Reconnect + catch-up by `last_acked_seq` | kill node mid-stream → zero loss, zero dupe |
| 5 | Presence + typing + read receipts | presence propagation < 2 s |
| 6 | Redis Streams durability + offline queue | broker restart → no acked-message loss |
| 7 | Next.js UI | the 4 demo GIFs (section 8) |
| 8 (stretch) | k8s 5-node scale test / Scylla migration write-up | 50k conns held 10 min |

Test the backend with a throwaway CLI / `wscat` client **before** any React.

---

## 8. UI Plan

UI is thin on purpose — it must *prove* the hard parts (ordering, reconnect, presence), not win design awards.

### Layout (Slack-shaped, 3 panels)
```
┌──────────┬─────────────────────────┐
│ convos   │  message list (scroll)  │
│ + unread │                         │
│ + presence│ ───────────────────── │
│ dots     │  [typing…]  [composer]  │
└──────────┴─────────────────────────┘
   ↑ connection badge: ● connected / ◐ reconnecting / ○ down
```

### 4 components that matter
1. **`useChatSocket` hook** — owns the WebSocket: auth handshake, exponential backoff reconnect (`isClosed` ref, cap 30 s), resume cursors on reopen. The brain.
2. **`MessageList`** — renders by `seq`, not arrival order; dedupe by `seq`. Proves ordering.
3. **`Composer`** — generates `client_msg_id` (uuid), optimistic grey bubble → confirmed on `ACK {seq}`. Proves at-least-once + dedupe.
4. **`ConnectionBadge`** — connected/reconnecting/down. When you kill a node in the demo, badge flips and **no messages disappear**. The screenshot that sells the project.

### Client state
```ts
interface ChatState {
  status: 'connecting' | 'connected' | 'reconnecting' | 'down';
  conversations: Record<string, Conversation>;
  messages: Record<string, Message[]>;   // keyed by conversationId, sorted by seq
  cursors: Record<string, number>;        // last_acked_seq per conversation
  presence: Record<string, Presence>;
}
```

### Demo GIFs (put in README)
- Optimistic send → grey → confirmed.
- Two tabs, live fan-out < 200 ms.
- Kill backend node → badge flips → messages survive → catch-up replays the gap.
- Presence/typing dots live.

### Do NOT build
Login/signup screens (hardcode 2–3 test users + "log in as" dropdown), emoji picker, themes, avatar upload, mobile polish beyond "doesn't break."

---

## 9. GitHub Repository — Full Setup

### Create the repo
```powershell
cd C:\Users\tdmne\Desktop\experiments\relay
git init -b main
gh repo create relay --public --source=. --description "Horizontally-scalable real-time messaging backend (Go + Redis + Postgres). 50k concurrent WS, fan-out p99 < 200ms, zero loss on node kill."
```

### Repo settings to configure (via GitHub UI or `gh`)
- **Default branch:** `main`.
- **Branch protection on `main`:**
  - Require pull request before merging (1 approval; solo-project exception: self-merge after a 24h review window).
  - Require status checks to pass (CI: build, lint, test).
  - Require branches up to date before merge.
  - Require conversation resolution.
  - Block force-pushes and deletions.
- **Security features (Settings → Code security):**
  - Dependabot alerts: **on**.
  - Dependabot security updates: **on**.
  - Secret scanning + push protection: **on**.
  - CodeQL code scanning: **on** (workflow in section 12).
- **Topics:** `distributed-systems`, `websocket`, `golang`, `real-time`, `redis`, `postgres`, `chat`.
- **About:** one-line description + the headline metric.

### `.github/` contents
```
.github/
├── workflows/
│   ├── ci.yml                # build + lint + test on PR
│   ├── codeql.yml            # security scanning
│   └── release.yml           # tag → changelog/release
├── ISSUE_TEMPLATE/
│   ├── bug_report.md
│   ├── feature_request.md
│   └── config.yml
├── PULL_REQUEST_TEMPLATE.md
├── dependabot.yml
└── CODEOWNERS
```

---

## 10. Required Repo Files (with templates)

### `LICENSE` — MIT
Use the standard MIT text. Generate with `gh repo create` license flag or paste the canonical MIT license with your name + year.

### `README.md` (top-of-file shape)
```markdown
# Relay

Horizontally-scalable real-time messaging backend.

**Sustains 50k concurrent WebSocket connections / 5 nodes · fan-out p99 < 200 ms · 10k msgs/sec · zero message loss on single-node kill.**

[architecture diagram] · [benchmarks] · [demo GIFs]

## Why
The hard part is keeping one message correct and fast when sender and receiver
sit on different machines: cross-node fan-out, per-conversation ordering,
at-least-once delivery, gap-free reconnect, presence at scale.

## Stack
Go gateway · Redis (pub/sub + streams) · Postgres · Next.js client.

## Run locally (Windows, no Docker)
...
## Reproduce the numbers
...
## Architecture
...
## Roadmap / out of scope
...
```

### `SECURITY.md`
```markdown
# Security Policy

## Supported Versions
The latest `main` and most recent tagged release receive security fixes.

## Reporting a Vulnerability
Do **not** open a public issue for security reports.
Use GitHub's private vulnerability reporting (Security → Report a vulnerability),
or email <your-email>. Expect an acknowledgement within 72 hours.

## Scope
- Auth handshake (JWT validation, expiry)
- Message authorization (membership checks)
- Rate limiting and input validation
- Dependency vulnerabilities

## Handling
Confirmed issues are fixed on a private branch, released, then disclosed.
```

### `CONTRIBUTING.md`
```markdown
# Contributing to Relay

## Before you start
- Read the architecture in `docs/architecture.md`.
- Open an issue describing the change before large PRs.

## Dev setup
See README → "Run locally". Requires Go, Node LTS, Postgres, Redis.

## Workflow
1. Branch from `main`: `feature/<desc>` or `fix/<desc>`.
2. Write tests first (TDD). No implementation without a test.
3. Run the gates locally before pushing:
   - Go: `go build ./... && go vet ./... && go test ./...`
   - Web: `npm run build && npm run lint && npm test`
4. Conventional Commits (see below).
5. Open a PR using the template. CI must be green.

## Commit format
`<type>(<scope>): <description>` — types: feat, fix, docs, style, refactor, test, chore, perf, ci.

## Code rules
- Gateway holds no global state.
- Sequencer is the only writer of `seq`.
- Every external call has a timeout (default 5s, max 30s) + circuit breaker.
- File size limit 300 lines; split by responsibility.
- No secrets in code or logs; never log message bodies.
```

### `CODE_OF_CONDUCT.md`
Use the **Contributor Covenant v2.1** (standard text). Fill in the contact method (private vulnerability reporting or email) for enforcement reports.

### `CHANGELOG.md`
Follow **Keep a Changelog** + **Semantic Versioning**:
```markdown
# Changelog
All notable changes documented here. Format: Keep a Changelog; versioning: SemVer.

## [Unreleased]
### Added
- ...

## [0.1.0] - YYYY-MM-DD
### Added
- Single-node gateway, echo, 10k conns/node.
```

### `.github/PULL_REQUEST_TEMPLATE.md`
```markdown
## What
<one-line summary>

## Why
<problem / context>

## How
<approach, tradeoffs>

## Measurable (if a phase milestone)
<number recorded in docs/benchmarks.md>

## Checklist
- [ ] Tests added/updated and passing
- [ ] `go build && go vet && go test` green
- [ ] No secrets committed
- [ ] Docs/CHANGELOG updated if needed
```

### `.github/ISSUE_TEMPLATE/bug_report.md` & `feature_request.md`
Standard GitHub templates — bug: steps to reproduce, expected vs actual, env (Windows/WSL, versions), logs (no secrets). Feature: problem, proposed solution, alternatives, the number it would move.

### `.github/dependabot.yml`
```yaml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/gateway"
    schedule: { interval: "weekly" }
  - package-ecosystem: "npm"
    directory: "/web"
    schedule: { interval: "weekly" }
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule: { interval: "weekly" }
```

### `.github/CODEOWNERS`
```
* @Surge77
```

### `.gitignore`
Ignore: `.env`, `.env.*` (keep `.env.example`), `node_modules/`, `/web/.next/`, Go build output/binaries, `*.log`, `*.pem`, `*.key`, `*.pfx`, `.DS_Store`, IDE folders. **Never** ignore `go.sum` / `package-lock.json`.

### `.env.example`
Same keys as section 6 with placeholder values (no real secrets).

---

## 11. Branching, Commit & Push Rules

### Branching
- Never commit directly to `main`.
- `feature/<short-desc>`, `fix/<short-desc>`, `hotfix/<short-desc>`.
- One logical change per branch.

### Commits (Conventional Commits)
```
<type>(<scope>): <description>

feat(gateway): assign per-conversation seq via redis INCR
fix(fanout): dedupe replayed messages on reconnect
perf(history): batch postgres checkpoint of sequencer
docs(readme): record phase-3 fan-out p99 numbers
test(sequencer): property test for monotonic seq
ci: add codeql scanning workflow
```
- Each commit = one logical change. Do not batch unrelated changes.
- Run linter + tests before committing.
- Never use "WIP"/"fix"/"update" as the whole message.
- Never `--no-verify` unless the hook itself is broken.
- Never amend an already-pushed commit on a shared branch.

### Push cadence ("push regularly")
- Push your working branch **at least daily**, and after every green local gate.
- Push when a phase milestone + its measurable is recorded.
- Open a **draft PR early** so CI runs continuously; mark ready when the phase is done.
- Keep branches short-lived (rebase on `main` frequently to avoid drift).

### Merge strategy
- PR required before merge. CI green + 1 approval (solo: self-merge after 24h window — PR still used for the diff record).
- `git merge --no-ff` when branch commits are individually meaningful; squash when history is noisy WIP. Don't mix both on one branch.
- Resolve conflicts by understanding both sides; rebase feature branches onto `main` rather than merging `main` in.

### Release tagging
- SemVer `v<major>.<minor>.<patch>`.
- Tag at each meaningful milestone: `git tag -a v0.1.0 -m "Phase 1 — single-node gateway, 10k conns"`.
- Push tags explicitly: `git push origin --tags`. Update `CHANGELOG.md` per release.

---

## 12. CI/CD & Automation

### `.github/workflows/ci.yml` (outline)
- Triggers: PRs to `main`, pushes to feature branches.
- Jobs:
  - **gateway:** `go build ./... && go vet ./... && go test ./... -race -cover`
  - **web:** `npm ci && npm run build && npm run lint && npm test`
  - Spin up Postgres + Redis as **GitHub Actions service containers** (this is fine in CI even though local is Docker-free — CI is Linux).
- Cache Go modules + npm.

### `.github/workflows/codeql.yml`
- CodeQL for `go` and `javascript-typescript` on a weekly schedule + PR.

### `.github/workflows/release.yml`
- On tag `v*`: build artifacts, generate release notes from `CHANGELOG.md`, create GitHub Release.

### Optional local hooks (pre-commit)
- Block commits containing `.env`, `*.pem`, `*.key`.
- Run `go vet` + `gofmt -l` and fail on diff.
- Keep hooks under 2 s; mechanical checks only.

---

## 13. Security Policy & Practices

### Application security (from the spec)
- Short-lived JWT (≤ 15 min) on the WS handshake; reject before processing any frame. Refresh via REST.
- Authorize every `SEND`/`READ` against `memberships` — never trust client-supplied membership.
- Validate + size-cap message body (≤ 16 KB); reject oversized frames.
- Rate-limit per connection (20 msgs/s) and per IP at the LB (token bucket).
- TLS (`wss://`) end-to-end; validate WS `Origin` on upgrade.
- No secrets in code/logs; never log message bodies in prod.

### Repo / supply-chain security
- Secret scanning + push protection **on**.
- Dependabot alerts + updates **on**; review weekly.
- CodeQL scanning **on**.
- Lock files committed (`go.sum`, `package-lock.json`); pin direct dependency versions.
- Before adding a dependency: check last commit date, open CVEs, downloads, license. Run `npm audit` / `govulncheck`.
- Private vulnerability reporting enabled; documented in `SECURITY.md`.

---

## 14. Maintenance Plan

- **Weekly:** review Dependabot PRs; merge safe updates after CI passes.
- **Per release:** update `CHANGELOG.md`, tag, push tags, cut a GitHub Release.
- **Issue hygiene:** triage with labels (`bug`, `enhancement`, `good first issue`, `security`, `docs`). Use issue templates.
- **Docs sync:** every architecture change updates `docs/architecture.md` + diagram in the same PR.
- **Benchmark freshness:** re-run `load/` and refresh `docs/benchmarks.md` whenever the message path changes.
- **Dependency policy:** reject packages with no commit in 12+ months unless intentionally frozen.
- **Stale branches:** delete merged branches; prune `[gone]` branches periodically.

---

## 15. Testing Strategy

- **Unit:** sequencer monotonicity, dedupe logic, membership authz, catch-up gap computation. Stub Redis/Postgres at the boundary — no real network in unit tests.
- **Integration:** two gateway nodes + real Redis → assert cross-node delivery + ordering.
- **Chaos:** kill a node mid-stream → assert zero loss / zero dupe. *This is the headline test.*
- **Load:** k6 / Go harness ramping to 50k conns → capture p50/p99 fan-out + history latency.
- **Property test:** for random interleaved sends, per-conversation `seq` is strictly increasing and gap-free per consumer.
- **Coverage:** 80% minimum on new logic; 100% on auth, validation, and the sequencer.
- **Type/lint gates:** `go vet` + `gofmt`; `tsc --noEmit` + ESLint for web.

---

## 16. Acceptance Criteria (definition of done)

- [ ] 50k concurrent conns across 5 nodes held stable 10 min.
- [ ] Fan-out p99 < 200 ms, history read p99 < 100 ms under 10k msgs/sec.
- [ ] Kill any gateway node → its clients reconnect < 5 s, zero message loss, zero duplicate delivered.
- [ ] Offline user receives all missed messages, in order, on reconnect.
- [ ] Per-conversation ordering holds under concurrent multi-sender load (property test green).
- [ ] README states the achieved numbers + the exact load-test command to reproduce them.
- [ ] All repo hygiene files present; branch protection + security features enabled; CI green.

---

## 17. Milestone Timeline

| Week | Milestone | Tag |
|------|-----------|-----|
| 1 | Repo + hygiene + CI (Phase 0) · single-node gateway (Phase 1) | `v0.1.0` |
| 2 | Metadata + sequencer + history (Phase 2) · fan-out multi-node (Phase 3) | `v0.2.0` |
| 3 | Reconnect/catch-up (Phase 4) · presence/typing/receipts (Phase 5) | `v0.3.0` |
| 4 | Redis Streams durability (Phase 6) · Next.js UI (Phase 7) | `v0.4.0` |
| 5 (stretch) | Scale test / Scylla write-up (Phase 8) · README numbers + GIFs | `v1.0.0` |

~3–5 weeks depending on depth of the scale demo.

---

## 18. Out of Scope (v1)

- Multi-region / geo-replication.
- End-to-end encryption (Signal protocol).
- Media / file attachments (text only v1).
- Message editing / deletion / threads.
- Push notifications to mobile OS.
- Channels > 1,000 members (document the path, don't build it).
- Kafka / Cassandra-Scylla (documented scale-out path, not built in v1).

---

*Plan generated for the "Distributed Chat Platform → strongest real-time systems signal" entry from the Notion "Projects to Build" master list. Page motto enforced throughout: every requirement carries a number.*
