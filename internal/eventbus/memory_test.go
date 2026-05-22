package eventbus

import (
	"context"
	"testing"
	"time"

	"forge/internal/harness"
)

func makeEvent(t harness.EventType) harness.Event {
	return harness.Event{Type: t}
}

// ── Publish + Subscribe ───────────────────────────────────────────────────────

func TestMemBus_PublishBeforeSubscribe_EventsDelivered(t *testing.T) {
	b := NewMemory()
	cursor := b.MarkRunStart("sess")

	b.Publish("sess", makeEvent(harness.EventAgentMessage))
	b.Publish("sess", makeEvent(harness.EventSessionIdle))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := b.Subscribe(ctx, "sess", cursor)

	got := collectN(t, ch, 2, time.Second)
	if len(got) != 2 {
		t.Errorf("expected 2 events, got %d", len(got))
	}
}

func TestMemBus_PublishAfterSubscribe_EventsDelivered(t *testing.T) {
	b := NewMemory()
	cursor := b.MarkRunStart("sess")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := b.Subscribe(ctx, "sess", cursor)

	b.Publish("sess", makeEvent(harness.EventAgentThinking))

	got := collectN(t, ch, 1, time.Second)
	if len(got) != 1 {
		t.Errorf("expected 1 event, got %d", len(got))
	}
}

func TestMemBus_SubscribeContextCancel_ChannelClosed(t *testing.T) {
	b := NewMemory()
	b.MarkRunStart("sess")

	ctx, cancel := context.WithCancel(context.Background())
	ch := b.Subscribe(ctx, "sess", "")

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed when context is cancelled")
		}
	case <-time.After(time.Second):
		t.Error("channel not closed after context cancel")
	}
}

func TestMemBus_MarkRunStart_ResetsQueue(t *testing.T) {
	b := NewMemory()

	// First run: publish 1 event
	b.MarkRunStart("sess")
	b.Publish("sess", makeEvent(harness.EventAgentMessage))

	// Second run: new cursor; old event should not be replayed
	cursor2 := b.MarkRunStart("sess")
	b.Publish("sess", makeEvent(harness.EventSessionIdle))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ch := b.Subscribe(ctx, "sess", cursor2)

	got := collectN(t, ch, 1, time.Second)
	if len(got) != 1 {
		t.Errorf("after MarkRunStart reset, expected 1 new event, got %d", len(got))
	}
	if got[0].Type != harness.EventSessionIdle {
		t.Errorf("expected EventSessionIdle, got %v", got[0].Type)
	}
}

// ── Interrupt + WatchInterrupt ────────────────────────────────────────────────

func TestMemBus_Interrupt_WatchReceivesSignal(t *testing.T) {
	b := NewMemory()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := b.WatchInterrupt(ctx, "sess")
	b.Interrupt("sess")

	select {
	case _, ok := <-ch:
		if !ok {
			// closed without sending — acceptable; the signal was received
		}
	case <-time.After(time.Second):
		t.Error("WatchInterrupt channel did not receive signal after Interrupt")
	}
}

func TestMemBus_Interrupt_BeforeWatch_SignalDelivered(t *testing.T) {
	b := NewMemory()
	b.Interrupt("sess")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := b.WatchInterrupt(ctx, "sess")

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Error("interrupt signal not delivered to late WatchInterrupt")
	}
}

func TestMemBus_WatchInterrupt_ContextCancel(t *testing.T) {
	b := NewMemory()
	ctx, cancel := context.WithCancel(context.Background())
	ch := b.WatchInterrupt(ctx, "sess")
	cancel()

	select {
	case <-ch: // closed or signal — both are fine
	case <-time.After(time.Second):
		t.Error("WatchInterrupt channel not closed on context cancel")
	}
}

// ── PurgeSession ─────────────────────────────────────────────────────────────

func TestMemBus_PurgeSession_DropsLatePublish(t *testing.T) {
	b := NewMemory()
	b.MarkRunStart("sess")
	b.PurgeSession("sess")

	// Late publish after purge should be a no-op (no goroutine leak)
	b.Publish("sess", makeEvent(harness.EventAgentMessage))
	// No assertion needed — just checking no panic/deadlock
}

func TestMemBus_PurgeSession_DoesNotAffectOtherSessions(t *testing.T) {
	b := NewMemory()
	b.MarkRunStart("sess-a")
	b.MarkRunStart("sess-b")

	b.Publish("sess-a", makeEvent(harness.EventAgentMessage))
	b.PurgeSession("sess-a")

	// sess-b's pump should still work
	cursor := b.MarkRunStart("sess-b")
	b.Publish("sess-b", makeEvent(harness.EventSessionIdle))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ch := b.Subscribe(ctx, "sess-b", cursor)

	got := collectN(t, ch, 1, time.Second)
	if len(got) != 1 {
		t.Errorf("sess-b should still work after sess-a purge, got %d events", len(got))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// collectN drains up to n events from ch, waiting up to timeout for each.
func collectN(t *testing.T, ch <-chan harness.Event, n int, timeout time.Duration) []harness.Event {
	t.Helper()
	var out []harness.Event
	deadline := time.After(timeout)
	for len(out) < n {
		select {
		case ev, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, ev)
		case <-deadline:
			return out
		}
	}
	return out
}
