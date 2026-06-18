-- Conversation management: names, ownership, DM de-duplication, and member roles.
-- conversations.kind already exists (channel|dm|group).
ALTER TABLE conversations
    ADD COLUMN name       text,
    ADD COLUMN created_by text REFERENCES users (id),
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
    -- dm_key is the sorted "userA:userB" pair for direct messages, UNIQUE so a DM
    -- between two users is created at most once. NULL for channels/groups.
    ADD COLUMN dm_key text UNIQUE;

ALTER TABLE memberships
    ADD COLUMN role        text NOT NULL DEFAULT 'member', -- owner | admin | member
    ADD COLUMN muted_until timestamptz;
