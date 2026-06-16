-- Dev/demo seed: three test users in one channel so the CLI and UI work without
-- a signup flow. Idempotent — safe to re-run. Remove for a production deployment.
INSERT INTO users (id, display_name) VALUES
    ('alice', 'Alice'),
    ('bob',   'Bob'),
    ('carol', 'Carol')
ON CONFLICT (id) DO NOTHING;

INSERT INTO conversations (id, kind) VALUES
    ('general', 'channel')
ON CONFLICT (id) DO NOTHING;

INSERT INTO memberships (conversation_id, user_id) VALUES
    ('general', 'alice'),
    ('general', 'bob'),
    ('general', 'carol')
ON CONFLICT (conversation_id, user_id) DO NOTHING;
