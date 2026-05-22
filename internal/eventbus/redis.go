package eventbus

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"forge/internal/harness"
)

const (
	redisStreamPrefix    = "forge:stream:"    // forge:stream:{sessionID}
	redisInterruptPrefix = "forge:ctrl:"      // forge:ctrl:{sessionID}:interrupt
	redisStreamMaxLen    = 1000               // approximate cap; old entries trimmed by XADD
	redisXReadBlock      = 5 * time.Second    // XREAD block timeout; loop re-checks ctx
	redisInterruptTTL    = 30 * time.Second
)

// redisBus implements EventBus using Redis Streams (XADD / XREAD BLOCK).
//
// Unlike the previous pub/sub implementation, events are persisted in the
// stream until the session is purged. A subscriber that connects after events
// were published receives them in order — there is no timing race between
// Publish and Subscribe.
//
// One stream per session: forge:stream:{sessionID}
// Events are capped at redisStreamMaxLen via XADD MAXLEN ~.
type redisBus struct {
	rdb *redis.Client
}

// NewRedis returns an EventBus backed by Redis Streams.
func NewRedis(addr, password string, db int) (EventBus, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &redisBus{rdb: rdb}, nil
}

// MarkRunStart returns the ID of the last entry currently in the stream, so
// that Subscribe(cursor) starts reading from the first event of the new run.
// Returns "0-0" when the stream is empty (subscriber reads everything).
func (b *redisBus) MarkRunStart(sessionID string) string {
	key := redisStreamPrefix + sessionID
	msgs, err := b.rdb.XRevRangeN(context.Background(), key, "+", "-", 1).Result()
	if err != nil || len(msgs) == 0 {
		return "0-0"
	}
	return msgs[0].ID
}

// Publish appends ev to the session's stream. XADD is non-blocking and never
// waits for a consumer. MAXLEN ~ caps the stream to avoid unbounded growth.
func (b *redisBus) Publish(sessionID string, ev harness.Event) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	key := redisStreamPrefix + sessionID
	args := &redis.XAddArgs{
		Stream: key,
		MaxLen: redisStreamMaxLen,
		Approx: true, // ~ trimming; more efficient than exact
		Values: map[string]any{"event": string(data)},
	}
	if err := b.rdb.XAdd(context.Background(), args).Err(); err != nil {
		slog.Warn("eventbus: redis xadd failed", "session", sessionID, "error", err)
	}
}

// Subscribe returns a channel that delivers all stream entries with ID greater
// than fromCursor (the value returned by MarkRunStart). Entries published
// before Subscribe is called are replayed first, then new entries follow.
// The channel is closed when ctx is cancelled.
func (b *redisBus) Subscribe(ctx context.Context, sessionID string, fromCursor string) <-chan harness.Event {
	if fromCursor == "" {
		fromCursor = "0-0"
	}
	out := make(chan harness.Event, 64)
	go func() {
		defer close(out)
		key := redisStreamPrefix + sessionID
		lastID := fromCursor
		for {
			msgs, err := b.rdb.XRead(ctx, &redis.XReadArgs{
				Streams: []string{key, lastID},
				Count:   100,
				Block:   redisXReadBlock,
			}).Result()
			if err != nil {
				if ctx.Err() != nil {
					return // context cancelled — normal shutdown
				}
				if err == redis.Nil {
					continue // timeout with no new messages; loop and re-check ctx
				}
				slog.Warn("eventbus: redis xread failed", "session", sessionID, "error", err)
				continue
			}
			for _, stream := range msgs {
				for _, msg := range stream.Messages {
					lastID = msg.ID
					raw, ok := msg.Values["event"].(string)
					if !ok {
						continue
					}
					var ev harness.Event
					if err := json.Unmarshal([]byte(raw), &ev); err != nil {
						slog.Debug("eventbus: unmarshal event", "error", err)
						continue
					}
					select {
					case out <- ev:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return out
}

func (b *redisBus) Interrupt(sessionID string) {
	key := redisInterruptPrefix + sessionID + ":interrupt"
	pipe := b.rdb.Pipeline()
	pipe.Set(context.Background(), key, "1", redisInterruptTTL)
	pipe.Publish(context.Background(), key, "1")
	if _, err := pipe.Exec(context.Background()); err != nil {
		slog.Warn("eventbus: redis interrupt failed", "session", sessionID, "error", err)
	}
}

func (b *redisBus) WatchInterrupt(ctx context.Context, sessionID string) <-chan struct{} {
	key := redisInterruptPrefix + sessionID + ":interrupt"
	out := make(chan struct{}, 1)
	go func() {
		defer close(out)
		sub := b.rdb.Subscribe(ctx, key)
		defer sub.Close()
		msgCh := sub.Channel()

		// Fast path: signal was already set before we subscribed.
		if n, err := b.rdb.Del(ctx, key).Result(); err == nil && n > 0 {
			out <- struct{}{}
			return
		}

		select {
		case _, ok := <-msgCh:
			if ok {
				b.rdb.Del(context.Background(), key) //nolint:errcheck
				out <- struct{}{}
			}
		case <-ctx.Done():
		}
	}()
	return out
}

// PurgeSession deletes the event stream and any interrupt state for sessionID.
func (b *redisBus) PurgeSession(sessionID string) {
	pipe := b.rdb.Pipeline()
	pipe.Del(context.Background(), redisStreamPrefix+sessionID)
	pipe.Del(context.Background(), redisInterruptPrefix+sessionID+":interrupt")
	if _, err := pipe.Exec(context.Background()); err != nil {
		slog.Warn("eventbus: purge session failed", "session", sessionID, "error", err)
	}
}
