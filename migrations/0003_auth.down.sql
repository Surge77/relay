DROP TABLE IF EXISTS refresh_tokens;

ALTER TABLE users
    DROP COLUMN IF EXISTS email,
    DROP COLUMN IF EXISTS password_hash,
    DROP COLUMN IF EXISTS avatar_url,
    DROP COLUMN IF EXISTS status_text,
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS updated_at;
