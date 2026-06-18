-- Identity for real authentication. users gains login credentials + profile
-- fields; refresh_tokens backs JWT access/refresh rotation. The pre-existing
-- seed users (alice/bob/carol) keep working with NULL email/password — they are
-- dev-only and never log in through the credential flow.
ALTER TABLE users
    ADD COLUMN email         text UNIQUE,
    ADD COLUMN password_hash text,
    ADD COLUMN avatar_url    text,
    ADD COLUMN status_text   text,
    ADD COLUMN created_at    timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN updated_at    timestamptz NOT NULL DEFAULT now();

-- One row per issued refresh token. We store only the SHA-256 of the opaque
-- token, never the token itself, so a database leak cannot mint sessions.
-- Rotation revokes the old row and inserts a new one; logout revokes.
CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    user_agent text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user ON refresh_tokens (user_id);
