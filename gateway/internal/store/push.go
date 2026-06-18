package store

import (
	"context"
	"fmt"

	"github.com/Surge77/relay/gateway/internal/model"
)

// SavePushSubscription stores (or refreshes) a web-push subscription.
func (s *Store) SavePushSubscription(ctx context.Context, userID, endpoint, p256dh, auth string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth_key) VALUES ($1,$2,$3,$4)
		 ON CONFLICT (user_id, endpoint) DO UPDATE SET p256dh=EXCLUDED.p256dh, auth_key=EXCLUDED.auth_key`,
		userID, endpoint, p256dh, auth)
	if err != nil {
		return fmt.Errorf("save push subscription: %w", err)
	}
	return nil
}

// DeletePushSubscription removes a subscription.
func (s *Store) DeletePushSubscription(ctx context.Context, userID, endpoint string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM push_subscriptions WHERE user_id=$1 AND endpoint=$2`, userID, endpoint)
	if err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	return nil
}

// PushSubscriptionsFor returns all of a user's push subscriptions.
func (s *Store) PushSubscriptionsFor(ctx context.Context, userID string) ([]model.PushSubscription, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_id, endpoint, p256dh, auth_key FROM push_subscriptions WHERE user_id=$1`, userID)
	if err != nil {
		return nil, fmt.Errorf("push subscriptions: %w", err)
	}
	defer rows.Close()
	var out []model.PushSubscription
	for rows.Next() {
		var p model.PushSubscription
		if err := rows.Scan(&p.UserID, &p.Endpoint, &p.P256dh, &p.Auth); err != nil {
			return nil, fmt.Errorf("scan push subscription: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
