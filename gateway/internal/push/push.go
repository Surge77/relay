// Package push delivers notifications to a user's offline devices. The transport
// is abstracted behind Notifier: the default LogNotifier records the intent, and
// a real VAPID/web-push sender drops in behind the same interface once its
// dependency is vetted. Triggering lives off the hot path (the durable stream
// consumer), so message latency is unaffected.
package push

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/Surge77/relay/gateway/internal/model"
)

// Notifier sends one payload to one subscription endpoint.
type Notifier interface {
	Notify(ctx context.Context, sub model.PushSubscription, payload []byte) error
}

// LogNotifier records the push instead of sending it — the dependency-free
// default. Swap for a VAPID sender in production.
type LogNotifier struct{}

func (LogNotifier) Notify(_ context.Context, sub model.PushSubscription, payload []byte) error {
	slog.Info("push (stub)", "endpoint", sub.Endpoint, "payload", string(payload))
	return nil
}

// Store is the lookup surface the service needs.
type Store interface {
	MembersOf(ctx context.Context, conversationID string) ([]string, error)
	PushSubscriptionsFor(ctx context.Context, userID string) ([]model.PushSubscription, error)
}

// Presence reports whether a user has a live connection.
type Presence interface {
	IsOnline(ctx context.Context, userID string) (bool, error)
}

// Service decides who to notify and fans payloads to their subscriptions.
type Service struct {
	store    Store
	presence Presence
	notifier Notifier
}

func NewService(store Store, presence Presence, notifier Notifier) *Service {
	return &Service{store: store, presence: presence, notifier: notifier}
}

// NotifyOfflineMembers notifies every member of a conversation, except the
// sender, who is currently offline. Best-effort: errors are swallowed so a
// failing endpoint never blocks message persistence. Production should further
// gate this on mention/DM to avoid channel noise.
func (s *Service) NotifyOfflineMembers(ctx context.Context, m model.Message) {
	members, err := s.store.MembersOf(ctx, m.ConversationID)
	if err != nil {
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"conversation_id": m.ConversationID,
		"sender_id":       m.SenderID,
		"preview":         m.Body,
	})
	for _, u := range members {
		if u == m.SenderID {
			continue
		}
		if online, _ := s.presence.IsOnline(ctx, u); online {
			continue
		}
		subs, err := s.store.PushSubscriptionsFor(ctx, u)
		if err != nil {
			continue
		}
		for _, sub := range subs {
			_ = s.notifier.Notify(ctx, sub, payload)
		}
	}
}
