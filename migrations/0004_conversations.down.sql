ALTER TABLE memberships
    DROP COLUMN IF EXISTS role,
    DROP COLUMN IF EXISTS muted_until;

ALTER TABLE conversations
    DROP COLUMN IF EXISTS name,
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS dm_key;
