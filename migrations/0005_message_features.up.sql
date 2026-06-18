-- Message editing/deletion, threaded replies, and reactions.
ALTER TABLE messages
    ADD COLUMN edited_at    timestamptz,
    ADD COLUMN deleted_at   timestamptz,            -- soft delete: row kept as a tombstone
    ADD COLUMN reply_to_seq bigint,                 -- thread parent within the same conversation
    ADD COLUMN kind         text NOT NULL DEFAULT 'text';

-- One row per (message, user, emoji). No FK to the partitioned messages table
-- (would require the partition key in the reference); integrity is enforced by
-- the application path that only reacts to existing messages.
CREATE TABLE reactions (
    conversation_id text   NOT NULL,
    message_seq     bigint NOT NULL,
    user_id         text   NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    emoji           text   NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (conversation_id, message_seq, user_id, emoji)
);

-- Mentions drive unread-mention counts and notifications.
CREATE TABLE mentions (
    conversation_id   text   NOT NULL,
    message_seq       bigint NOT NULL,
    mentioned_user_id text   NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (conversation_id, message_seq, mentioned_user_id)
);
