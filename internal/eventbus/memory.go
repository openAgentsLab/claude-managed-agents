package eventbus

import (
	"context"
	"sync"

	"forge/internal/harness"
)

// memBus is an in-process EventBus implementation backed by per-session queues.
// Events are buffered in an unbounded in-memory queue and are never dropped,
// solving the timing race where Publish fires before Subscribe is called.
// Suitable for single-node tests; production deployments should use NewRedis.
type memBus struct {
	mu     sync.Mutex
	pumps  map[string]*sessionPump
	purged map[string]struct{} // sessions explicitly purged; Publish must not create new pumps for these

	intMu      sync.Mutex
	interrupts map[string]chan struct{}
}

// NewMemory returns an in-process EventBus.
func NewMemory() EventBus {
	return &memBus{
		pumps:      make(map[string]*sessionPump),
		purged:     make(map[string]struct{}),
		interrupts: make(map[string]chan struct{}),
	}
}

// MarkRunStart resets the queue for sessionID and returns "" (cursor is unused
// for the in-memory implementation; the pump always starts empty).
func (b *memBus) MarkRunStart(sessionID string) string {
	p := newSessionPump()
	b.mu.Lock()
	old := b.pumps[sessionID]
	b.pumps[sessionID] = p
	// A new run clears the purged flag so Publish can buffer future events.
	delete(b.purged, sessionID)
	b.mu.Unlock()
	// Stop the old pump after releasing the lock to avoid holding b.mu during
	// a blocking channel drain.
	if old != nil {
		old.stop()
	}
	return ""
}

func (b *memBus) Publish(sessionID string, ev harness.Event) {
	b.mu.Lock()
	p, ok := b.pumps[sessionID]
	if !ok {
		// Do not create a pump for a purged session: the session has been
		// explicitly torn down and any late-arriving events should be dropped.
		// Without this guard, Publish would start a goroutine that leaks forever.
		if _, isPurged := b.purged[sessionID]; isPurged {
			b.mu.Unlock()
			return
		}
		// No active run yet; buffer events so they are not lost if Publish
		// races ahead of MarkRunStart.
		p = newSessionPump()
		b.pumps[sessionID] = p
	}
	b.mu.Unlock()
	p.push(ev)
}

// Subscribe returns a channel delivering events from the current run's queue.
// fromCursor is ignored for the in-memory implementation.
func (b *memBus) Subscribe(ctx context.Context, sessionID string, _ string) <-chan harness.Event {
	b.mu.Lock()
	p, ok := b.pumps[sessionID]
	if !ok {
		p = newSessionPump()
		b.pumps[sessionID] = p
	}
	b.mu.Unlock()

	out := make(chan harness.Event, 64)
	go func() {
		defer close(out)
		for {
			select {
			case ev, ok := <-p.out:
				if !ok {
					return
				}
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (b *memBus) Interrupt(sessionID string) {
	b.intMu.Lock()
	defer b.intMu.Unlock()
	ch, ok := b.interrupts[sessionID]
	if !ok {
		ch = make(chan struct{}, 1)
		b.interrupts[sessionID] = ch
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (b *memBus) WatchInterrupt(ctx context.Context, sessionID string) <-chan struct{} {
	b.intMu.Lock()
	if b.interrupts[sessionID] == nil {
		b.interrupts[sessionID] = make(chan struct{}, 1)
	}
	intCh := b.interrupts[sessionID]
	b.intMu.Unlock()

	out := make(chan struct{}, 1)
	go func() {
		defer close(out)
		select {
		case <-intCh:
			out <- struct{}{}
			b.intMu.Lock()
			delete(b.interrupts, sessionID)
			b.intMu.Unlock()
		case <-ctx.Done():
		}
	}()
	return out
}

func (b *memBus) PurgeSession(sessionID string) {
	b.mu.Lock()
	old := b.pumps[sessionID]
	delete(b.pumps, sessionID)
	b.purged[sessionID] = struct{}{}
	b.mu.Unlock()
	// Stop outside the lock to avoid holding b.mu during a blocking drain.
	if old != nil {
		old.stop()
	}

	b.intMu.Lock()
	delete(b.interrupts, sessionID)
	b.intMu.Unlock()
}

// ── sessionPump ───────────────────────────────────────────────────────────────

// sessionPump is a goroutine-based unbounded queue.
// push never blocks regardless of consumer speed.
type sessionPump struct {
	in      chan harness.Event
	out     chan harness.Event
	stopCh  chan struct{}
	stopped chan struct{}
}

func newSessionPump() *sessionPump {
	p := &sessionPump{
		in:      make(chan harness.Event, 64),
		out:     make(chan harness.Event, 64),
		stopCh:  make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go p.run()
	return p
}

func (p *sessionPump) push(ev harness.Event) {
	select {
	case p.in <- ev:
	case <-p.stopped:
	}
}

func (p *sessionPump) stop() {
	select {
	case <-p.stopCh: // already stopped
	default:
		close(p.stopCh)
	}
	<-p.stopped
}

func (p *sessionPump) run() {
	defer close(p.stopped)
	defer close(p.out)
	var buf []harness.Event
	for {
		if len(buf) == 0 {
			select {
			case ev := <-p.in:
				buf = append(buf, ev)
			case <-p.stopCh:
				return
			}
		} else {
			select {
			case ev := <-p.in:
				buf = append(buf, ev)
			case p.out <- buf[0]:
				buf = buf[1:]
			case <-p.stopCh:
				// Flush remaining buffered events before exiting.
				for _, ev := range buf {
					select {
					case p.out <- ev:
					default:
					}
				}
				return
			}
		}
	}
}
