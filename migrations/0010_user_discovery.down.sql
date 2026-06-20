ALTER TABLE users DROP COLUMN IF EXISTS last_seen_at;
DROP INDEX IF EXISTS idx_users_email_trgm;
DROP INDEX IF EXISTS idx_users_display_name_trgm;
-- pg_trgm extension is left installed; it is harmless and may be shared.
