-- Full-text search over message bodies using a generated tsvector + GIN index.
-- No external search engine. The generated column propagates to every hash
-- partition; the GIN index is created per-partition automatically.
ALTER TABLE messages
    ADD COLUMN tsv tsvector GENERATED ALWAYS AS (to_tsvector('english', body)) STORED;

CREATE INDEX messages_tsv ON messages USING GIN (tsv);
