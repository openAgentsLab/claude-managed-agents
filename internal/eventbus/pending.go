package eventbus

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"forge/internal/harness"
)

// deliverLua atomically checks and deletes the presence marker, then pushes the
// result onto the list.  Using a Lua script prevents a partial-delivery scenario
// where Del succeeds but LPush fails, leaving the caller unable to retry.
const deliverLua = `
local n = redis.call('DEL', KEYS[1])
if n == 0 then return 0 end
redis.call('LPUSH', KEYS[2], ARGV[1])
redis.call('EXPIRE', KEYS[2], ARGV[2])
return 1
`

const (
	redisPendingPrefix = "forge:pending:" // forge:pending:{actionID}      — result list
	redisPendingTTL    = 10 * time.Minute
)

// ─── MemoryPending ────────────────────────────────────────────────────────────

// MemoryPending is a single-node PendingStore backed by in-process channels.
type MemoryPending struct {
	m sync.Map // actionID → chan harness.SuspendedResult
}

// NewMemoryPending returns a single-node PendingStore.
func NewMemoryPending() harness.PendingStore {
	return &MemoryPending{}
}

func (p *MemoryPending) Register(ctx context.Context, actionID string) <-chan harness.SuspendedResult {
	ch := make(chan harness.SuspendedResult, 1)
	p.m.Store(actionID, ch)
	go func() {
		<-ctx.Done()
		// Win the race against Deliver: only close when we successfully remove the
		// entry (i.e. Deliver has not already consumed it).
		if _, deleted := p.m.LoadAndDelete(actionID); deleted {
			close(ch)
		}
	}()
	return ch
}

func (p *MemoryPending) Deliver(actionID string, result harness.SuspendedResult) bool {
	v, ok := p.m.LoadAndDelete(actionID)
	if !ok {
		return false
	}
	v.(chan harness.SuspendedResult) <- result
	return true
}

// ─── RedisPending ─────────────────────────────────────────────────────────────

// RedisPending is a multi-node PendingStore backed by Redis lists.
// Register stores a presence marker key and starts a BLPOP goroutine.
// Deliver atomically consumes the presence marker then LPUSHes the result.
type RedisPending struct {
	rdb *redis.Client
}

// NewRedisPending returns a multi-node PendingStore backed by Redis.
func NewRedisPending(addr, password string, db int) (harness.PendingStore, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &RedisPending{rdb: rdb}, nil
}

func (p *RedisPending) Register(ctx context.Context, actionID string) <-chan harness.SuspendedResult {
	ch := make(chan harness.SuspendedResult, 1)
	listKey := redisPendingPrefix + actionID
	regKey := redisPendingPrefix + actionID + ":reg"

	// Set presence marker so Deliver can confirm this slot is active.
	p.rdb.Set(ctx, regKey, "1", redisPendingTTL) //nolint:errcheck

	go func() {
		defer close(ch)
		// Block until a result is pushed by Deliver or ctx is cancelled.
		// go-redis respects ctx cancellation, so BLPop returns immediately when
		// ctx is cancelled without leaving a dangling connection.
		vals, err := p.rdb.BLPop(ctx, 0, listKey).Result()
		if err != nil {
			// ctx cancelled or Redis error — slot expires via TTL
			p.rdb.Del(context.Background(), regKey) //nolint:errcheck
			return
		}
		if len(vals) < 2 {
			return
		}
		var r harness.SuspendedResult
		if err := json.Unmarshal([]byte(vals[1]), &r); err != nil {
			slog.Warn("pending: unmarshal result", "action_id", actionID, "error", err)
			return
		}
		ch <- r
	}()
	return ch
}

func (p *RedisPending) Deliver(actionID string, result harness.SuspendedResult) bool {
	regKey := redisPendingPrefix + actionID + ":reg"
	listKey := redisPendingPrefix + actionID

	data, err := json.Marshal(result)
	if err != nil {
		return false
	}

	// Lua script atomically: check-and-delete presence marker, then LPUSH result.
	// This prevents the partial-delivery bug where Del succeeds but LPush fails,
	// which would consume the regKey and make the actionID unretryable.
	n, err := p.rdb.Eval(
		context.Background(), deliverLua,
		[]string{regKey, listKey},
		string(data), int(redisPendingTTL.Seconds()),
	).Int()
	if err != nil {
		slog.Warn("pending: redis deliver error", "action_id", actionID, "error", err)
		return false
	}
	return n == 1
}
