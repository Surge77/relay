package push

import (
	"context"
	"testing"

	"github.com/Surge77/relay/gateway/internal/model"
)

type fakeStore struct {
	members []string
	subs    map[string][]model.PushSubscription
}

func (f fakeStore) MembersOf(context.Context, string) ([]string, error) { return f.members, nil }
func (f fakeStore) PushSubscriptionsFor(_ context.Context, u string) ([]model.PushSubscription, error) {
	return f.subs[u], nil
}

type fakePresence struct{ online map[string]bool }

func (p fakePresence) IsOnline(_ context.Context, u string) (bool, error) { return p.online[u], nil }

type capturingNotifier struct{ sent []string }

func (c *capturingNotifier) Notify(_ context.Context, sub model.PushSubscription, _ []byte) error {
	c.sent = append(c.sent, sub.UserID)
	return nil
}

func TestNotifyOfflineMembers_SkipsSenderAndOnline(t *testing.T) {
	store := fakeStore{
		members: []string{"alice", "bob", "carol"},
		subs: map[string][]model.PushSubscription{
			"bob":   {{UserID: "bob", Endpoint: "e-bob"}},
			"carol": {{UserID: "carol", Endpoint: "e-carol"}},
		},
	}
	pres := fakePresence{online: map[string]bool{"carol": true}} // carol is online
	notif := &capturingNotifier{}
	svc := NewService(store, pres, notif)

	// alice sends. alice=sender (skip), bob offline+subscribed (notify),
	// carol online (skip).
	svc.NotifyOfflineMembers(context.Background(),
		model.Message{ConversationID: "c", SenderID: "alice", Body: "hi"})

	if len(notif.sent) != 1 || notif.sent[0] != "bob" {
		t.Fatalf("notified = %v, want exactly [bob]", notif.sent)
	}
}
