// Package eventbus provides per-session event queuing for harness events and
// interrupt signals. Two implementations are available:
//
//   - memory: in-process queue; suitable for single-node tests.
//   - redis:  uses Redis Streams (XADD/XREAD BLOCK); events are buffered in the
//     stream and delivered to late-connecting SSE subscribers, enabling
//     multi-node deployments without sticky sessions.
//
// Unlike pub/sub, events published before a subscriber connects are not lost:
// they remain in the queue until consumed.
package eventbus

import (
	"context"

	"forge/internal/harness"
)

// EventBus is the queued event delivery contract used by the HTTP orchestration layer.
type EventBus interface {
	// MarkRunStart records the current tail of the event stream for sessionID
	// and returns an opaque cursor. Pass this cursor to Subscribe so the new
	// subscriber receives every event from this run onwards — including events
	// that were published before the subscriber connected.
	// Must be called in handleRun before spawning the agent goroutine.
	MarkRunStart(sessionID string) string

	// Publish appends ev to the event queue for sessionID.
	// Never blocks: the in-memory implementation uses an unbounded internal
	// buffer; the Redis implementation uses XADD which is always non-blocking.
	Publish(sessionID string, ev harness.Event)

	// Subscribe returns a channel that receives events for sessionID starting
	// from fromCursor (the value returned by MarkRunStart). Events published
	// before Subscribe is called but after MarkRunStart are replayed first,
	// then live events follow. The channel is closed when ctx is cancelled.
	Subscribe(ctx context.Context, sessionID string, fromCursor string) <-chan harness.Event

	// Interrupt signals the running agent goroutine for sessionID to stop.
	// Works across nodes when the Redis backend is used.
	Interrupt(sessionID string)

	// WatchInterrupt returns a channel that is written to (and closed) once
	// when Interrupt is called for sessionID, or when ctx is cancelled.
	// Called by the running agent goroutine to honour cross-node interrupts.
	WatchInterrupt(ctx context.Context, sessionID string) <-chan struct{}

	// PurgeSession releases all resources associated with sessionID.
	// Called when a session is permanently deleted.
	PurgeSession(sessionID string)
}
