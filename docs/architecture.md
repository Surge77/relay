# Relay Architecture

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

## Message lifecycle (send)

1. Client → Gateway: `{client_msg_id, conversation_id, body}`.
2. Gateway validates membership + auth, calls the sequencer → assigns `seq`
   (`INCR conv:{id}:seq`, seeded from Postgres `MAX(seq)` on cold start).
3. **Durable first:** gateway appends to the Redis Stream `messages.persist`.
4. **Then live:** gateway publishes to Redis channel `conv:{id}`.
5. Every gateway subscribed to `conv:{id}` (hosting an online member) receives it and pushes to
   that socket.
6. A stream consumer writes the message to Postgres history and enqueues offline members.
7. Gateway acks `{client_msg_id → seq}` to the sender.

The order of steps 3 and 4 is the key correctness invariant: anything delivered live in step 4
was already made durable in step 3, so reconnect catch-up can always recover it.

## Reconnect / catch-up

The client sends `last_acked_seq` per conversation on `subscribe`. The gateway joins live
fan-out first (no gap), then replays history strictly after `last_acked_seq`. The client
dedupes any overlap by `seq`. Result: gap-free, duplicate-tolerant resume against any node.

## Sequencing & ordering

`seq` is per-conversation, strictly increasing, gap-free, and assigned by exactly one logical
sequencer (Redis `INCR`). Postgres is the durable source of truth: on a Redis key miss the
counter is re-seeded from `MAX(seq)` under `SETNX`, so a Redis restart cannot regress it and
produce duplicate sequence numbers.

## Why these components

- **Go gateway** — goroutine-per-connection gives the cleanest path to high socket density.
- **Redis pub/sub** — cheap cross-node routing keyed by conversation.
- **Redis Streams** — consumer-group durability + replay without operating Kafka.
- **Postgres** — relational metadata + partitioned history (`conversation_id`), clustered on
  `seq`. Migration path to Cassandra/Scylla is documented, not built, in v1.
