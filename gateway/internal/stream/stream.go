// Package stream is the durable async pipeline. Every message is appended to a
// Redis Stream (durable with AOF) and drained by a consumer group, giving
// at-least-once processing that survives a broker restart: unacknowledged
// entries remain pending and are re-delivered, so no acknowledged message is
// lost.
package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Surge77/relay/gateway/internal/model"
)

// StreamKey is the durable message stream. Group is the consumer group that
// drains it.
const (
	StreamKey = "messages.persist"
	Group     = "persisters"
)

// Log appends messages to the durable stream. It implements hub.Persister, so a
// message is durably recorded before it is published for live fan-out.
type Log struct {
	rdb *redis.Client
}

func NewLog(rdb *redis.Client) *Log { return &Log{rdb: rdb} }

// Persist appends the message to the stream as a single JSON field.
func (l *Log) Persist(ctx context.Context, m model.Message) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	if err := l.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamKey,
		Values: map[string]any{"msg": data},
	}).Err(); err != nil {
		return fmt.Errorf("xadd: %w", err)
	}
	return nil
}

// Sink receives each message drained from the stream. Returning an error leaves
// the entry unacknowledged so it is re-delivered (at-least-once).
type Sink func(ctx context.Context, m model.Message) error

// Consumer drains the durable stream into a Sink as part of a consumer group.
type Consumer struct {
	rdb      *redis.Client
	consumer string
	sink     Sink
	block    time.Duration
}

// NewConsumer creates a consumer group reader. consumerName identifies this node
// within the group so pending entries can be re-claimed after a restart.
func NewConsumer(rdb *redis.Client, consumerName string, sink Sink) *Consumer {
	return &Consumer{rdb: rdb, consumer: consumerName, sink: sink, block: 500 * time.Millisecond}
}

// EnsureGroup creates the consumer group (and the stream) if absent.
func (c *Consumer) EnsureGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, StreamKey, Group, "0").Err()
	if err != nil && !isGroupExists(err) {
		return fmt.Errorf("create group: %w", err)
	}
	return nil
}

// Run drains the stream until ctx is cancelled. It first reprocesses this
// consumer's pending (unacked) entries — the entries in flight when the process
// last died — then reads new ones.
func (c *Consumer) Run(ctx context.Context) error {
	if err := c.EnsureGroup(ctx); err != nil {
		return err
	}
	if err := c.drainPending(ctx); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := c.readBatch(ctx, ">"); err != nil && !errors.Is(err, context.Canceled) {
			// Transient read error: brief backoff, then retry — but stay responsive
			// to cancellation so shutdown isn't delayed by a full backoff interval.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.block):
			}
		}
	}
}

// drainPending reprocesses entries delivered to this consumer but never acked
// (i.e. in flight at the last crash) — the restart-durability path.
func (c *Consumer) drainPending(ctx context.Context) error {
	for {
		n, err := c.readBatchCount(ctx, "0")
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
	}
}

func (c *Consumer) readBatch(ctx context.Context, id string) error {
	_, err := c.readBatchCount(ctx, id)
	return err
}

// readBatchCount reads one batch from the given start id ("0" = pending, ">" =
// new), processes each via the sink, and acks the successes. It returns the
// number of entries read.
func (c *Consumer) readBatchCount(ctx context.Context, id string) (int, error) {
	res, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    Group,
		Consumer: c.consumer,
		Streams:  []string{StreamKey, id},
		Count:    64,
		Block:    c.block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("xreadgroup: %w", err)
	}

	count := 0
	for _, stream := range res {
		for _, msg := range stream.Messages {
			count++
			m, perr := decode(msg)
			if perr != nil {
				// Poison entry: ack it to stop redelivery; it can't be parsed.
				c.rdb.XAck(ctx, StreamKey, Group, msg.ID)
				continue
			}
			if err := c.sink(ctx, m); err != nil {
				continue // leave unacked → redelivered later
			}
			c.rdb.XAck(ctx, StreamKey, Group, msg.ID)
		}
	}
	return count, nil
}

func decode(msg redis.XMessage) (model.Message, error) {
	raw, ok := msg.Values["msg"].(string)
	if !ok {
		return model.Message{}, errors.New("missing msg field")
	}
	var m model.Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return model.Message{}, err
	}
	return m, nil
}

func isGroupExists(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}
