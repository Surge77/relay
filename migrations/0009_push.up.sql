-- Web-push subscriptions (one per browser/endpoint per user).
CREATE TABLE push_subscriptions (
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    endpoint   text NOT NULL,
    p256dh     text NOT NULL,
    auth_key   text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, endpoint)
);
