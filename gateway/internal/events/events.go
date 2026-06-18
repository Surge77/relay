// Package events publishes control-plane realtime frames onto the same Redis
// fan-out the gateway consumes: conversation-scoped frames go to the conversation
// channel, user-targeted frames to the recipient's per-user channel. It lets the
// stateless REST control plane notify connected clients without its own sockets.
package events

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/Surge77/relay/gateway/internal/fanout"
	"github.com/Surge77/relay/gateway/internal/protocol"
)

// Publisher emits frames to the gateway fan-out. It is publish-only — it reuses
// the fan-out's publish path and never subscribes.
type Publisher struct {
	fan *fanout.Redis
}

func NewPublisher(rdb *redis.Client) *Publisher {
	return &Publisher{fan: fanout.NewRedis(context.Background(), rdb)}
}

// ToConversation delivers a frame to every member of a conversation currently
// connected to any node.
func (p *Publisher) ToConversation(ctx context.Context, conversationID string, f protocol.Frame) error {
	return p.fan.Publish(ctx, conversationID, f)
}

// ToUser delivers a frame to a specific user's connections on any node.
func (p *Publisher) ToUser(ctx context.Context, userID string, f protocol.Frame) error {
	return p.fan.Publish(ctx, protocol.UserChannel(userID), f)
}
