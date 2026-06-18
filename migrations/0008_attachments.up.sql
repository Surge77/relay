-- File/image attachments. The bytes live in the configured Storage backend
-- (local disk in dev, S3-compatible in prod); this row is the metadata + key.
CREATE TABLE attachments (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id text   NOT NULL,
    uploader_id     text   NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    filename        text   NOT NULL,
    content_type    text   NOT NULL,
    size_bytes      bigint NOT NULL,
    storage_key     text   NOT NULL,
    width           int,
    height          int,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX attachments_conversation ON attachments (conversation_id);
