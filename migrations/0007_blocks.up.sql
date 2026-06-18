-- User blocking. Mutes already live on memberships.muted_until (migration 0004).
CREATE TABLE blocks (
    blocker_id text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    blocked_id text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id)
);
