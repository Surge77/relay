DROP TABLE IF EXISTS mentions;
DROP TABLE IF EXISTS reactions;

ALTER TABLE messages
    DROP COLUMN IF EXISTS edited_at,
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS reply_to_seq,
    DROP COLUMN IF EXISTS kind;
