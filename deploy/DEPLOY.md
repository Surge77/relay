# Deploying Relay (cheap single-node path)

Relay can't run entirely on Vercel: the **gateway** holds long-lived WebSocket
connections and a background Redis pub/sub subscriber, which serverless functions
can't host. So we split by statefulness:

| Piece | Host | Notes |
|-------|------|-------|
| web (Next.js) | **Vercel** | stateless; proxies `/api/*`, connects WS direct |
| gateway + api (Go) | **Fly.io** | always-on containers (this guide) |
| Postgres | **Neon** | serverless PG, free tier |
| Redis (pub/sub + streams) | **Upstash** | TCP endpoint, free tier — see caveat |

This is the **single-node** path: real and live, but one gateway machine, so the
cross-node fan-out path isn't exercised. To scale later: `fly scale count=2`
on `relay-gw` (the "any node serves any reconnect" design needs no sticky sessions).

Prereqecs: [`flyctl`](https://fly.io/docs/flyctl/install/), the `migrate` CLI, and
accounts on Fly, Neon, Upstash, Vercel.

---

## 1. Postgres (Neon)

1. Create a project at neon.tech → copy the **pooled** connection string.
2. Ensure it ends with `?sslmode=require`. Call it `$PG`.
3. Apply migrations from the repo root:
   ```sh
   migrate -path migrations -database "$PG" up
   ```

## 2. Redis (Upstash)

1. Create a Redis database at upstash.com → copy the **`rediss://` TCP URL**
   (not the REST URL). Call it `$REDIS`.
2. **Caveat:** Relay uses Redis **pub/sub _and_ streams**. Upstash supports both
   over TCP, but if you hit pub/sub limits on the free tier, the fallback is a
   self-hosted Redis on Fly:
   ```sh
   fly launch --image redis:7-alpine --name relay-redis --no-deploy
   fly volumes create relay_redis_data --size 1 -a relay-redis
   # mount it at /data, deploy; then REDIS=redis://relay-redis.internal:6379
   ```

## 3. Pick a shared secret

```sh
JWT=$(openssl rand -hex 32)   # same value for BOTH Go services
```

## 4. Gateway (Fly)

From the repo root:
```sh
fly apps create relay-gw
fly secrets set -a relay-gw \
  JWT_SECRET="$JWT" POSTGRES_URL="$PG" REDIS_URL="$REDIS" \
  ALLOWED_ORIGINS="https://PLACEHOLDER"      # set real Vercel domain in step 6
fly deploy gateway --config deploy/fly/gateway.fly.toml
```
Note the URL: `https://relay-gw.fly.dev` → WS endpoint is `wss://relay-gw.fly.dev/ws`.

## 5. API (Fly)

```sh
fly apps create relay-api
fly volumes create relay_uploads --size 1 -a relay-api   # for attachments
fly secrets set -a relay-api \
  JWT_SECRET="$JWT" POSTGRES_URL="$PG" REDIS_URL="$REDIS" \
  ALLOWED_ORIGINS="https://PLACEHOLDER"
fly deploy gateway --config deploy/fly/api.fly.toml
```
Verify: `curl https://relay-api.fly.dev/readyz` → `{"success":true,...}`.

## 6. Web (Vercel)

1. Import the repo on Vercel; set **Root Directory = `web`**.
2. Environment variables:
   - `API_ORIGIN=https://relay-api.fly.dev`
   - `NEXT_PUBLIC_GATEWAY_WS=wss://relay-gw.fly.dev/ws`
3. Deploy → note the domain, e.g. `https://relay-xyz.vercel.app`.

## 7. Close the CORS/WS-origin loop

Point both Go services at the real web domain, then redeploy is not needed
(secrets restart the machines):
```sh
fly secrets set -a relay-gw  ALLOWED_ORIGINS="https://relay-xyz.vercel.app"
fly secrets set -a relay-api ALLOWED_ORIGINS="https://relay-xyz.vercel.app"
```

## 8. Smoke test

Open the Vercel URL in two browsers → sign up two users → search one from the
other → start a DM → send a message; it should appear live. Close one tab → the
DM header flips to "last seen …".

---

## Gotchas
- **`wss://` is mandatory** — an HTTPS page can't open a `ws://` socket. Fly gives
  TLS for free via `force_https`.
- **`JWT_SECRET` must match** across gateway + api or tokens won't verify.
- **`auto_stop_machines = false`** (set in the toml) — Fly must not idle a machine
  holding open WebSocket connections.
- **Migrations run once** against Neon (step 1); the services don't self-migrate.
- **Cost:** one `shared-cpu-1x` 256MB machine each + Neon/Upstash free tiers ≈ a
  few dollars/month, often within Fly's allowance.
