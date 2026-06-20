-- User discovery: substring search over display name / email, plus last-seen
-- tracking so DMs can show "last seen …" for an offline peer.

-- Trigram support makes case-insensitive substring search (ILIKE '%q%')
-- index-backed instead of a full table scan.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_users_display_name_trgm
    ON users USING gin (lower(display_name) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_users_email_trgm
    ON users USING gin (lower(email) gin_trgm_ops);

ALTER TABLE users ADD COLUMN IF NOT EXISTS last_seen_at timestamptz;
