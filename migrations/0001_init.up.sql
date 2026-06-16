-- Core metadata.
CREATE TABLE users (
    id           text PRIMARY KEY,
    display_name text NOT NULL
);

CREATE TABLE conversations (
    id         text PRIMARY KEY,
    kind       text NOT NULL DEFAULT 'channel',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
    conversation_id text   NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    user_id         text   NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    last_read_seq   bigint NOT NULL DEFAULT 0,
    joined_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (conversation_id, user_id)
);

-- Message history, hash-partitioned by conversation_id so a hot conversation's
-- rows colocate and the table scales horizontally. The PK (conversation_id, seq)
-- enforces per-conversation ordering and gap-free sequencing; the unique dedupe
-- index makes Persist idempotent under at-least-once delivery.
CREATE TABLE messages (
    conversation_id text   NOT NULL,
    seq             bigint NOT NULL,
    sender_id       text   NOT NULL,
    client_msg_id   text   NOT NULL,
    body            text   NOT NULL,
    ts              bigint NOT NULL,
    PRIMARY KEY (conversation_id, seq)
) PARTITION BY HASH (conversation_id);

CREATE TABLE messages_p0 PARTITION OF messages FOR VALUES WITH (MODULUS 8, REMAINDER 0);
CREATE TABLE messages_p1 PARTITION OF messages FOR VALUES WITH (MODULUS 8, REMAINDER 1);
CREATE TABLE messages_p2 PARTITION OF messages FOR VALUES WITH (MODULUS 8, REMAINDER 2);
CREATE TABLE messages_p3 PARTITION OF messages FOR VALUES WITH (MODULUS 8, REMAINDER 3);
CREATE TABLE messages_p4 PARTITION OF messages FOR VALUES WITH (MODULUS 8, REMAINDER 4);
CREATE TABLE messages_p5 PARTITION OF messages FOR VALUES WITH (MODULUS 8, REMAINDER 5);
CREATE TABLE messages_p6 PARTITION OF messages FOR VALUES WITH (MODULUS 8, REMAINDER 6);
CREATE TABLE messages_p7 PARTITION OF messages FOR VALUES WITH (MODULUS 8, REMAINDER 7);

CREATE UNIQUE INDEX messages_dedupe ON messages (conversation_id, sender_id, client_msg_id);
