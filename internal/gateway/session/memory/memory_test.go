package memory

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"forge/internal/gateway/session"
)

// ── EmitEvent ────────────────────────────────────────────────────────────────

func TestEmitEvent_IdempotentByID(t *testing.T) {
	s := NewInMemoryStore()
	e := session.Event{ID: "evt-1", Role: "user", Content: "hello"}

	if _, err := s.EmitEvent("sess", e); err != nil {
		t.Fatalf("first emit: %v", err)
	}
	if _, err := s.EmitEvent("sess", e); err != nil {
		t.Fatalf("second emit (same ID): %v", err)
	}

	events, _ := s.GetEvents("sess")
	if len(events) != 1 {
		t.Errorf("expected 1 event after duplicate emit, got %d", len(events))
	}
}

func TestEmitEvent_AutoAssignsID(t *testing.T) {
	s := NewInMemoryStore()
	e := session.Event{Role: "user", Content: "hello"}

	if _, err := s.EmitEvent("sess", e); err != nil {
		t.Fatalf("emit: %v", err)
	}
	events, _ := s.GetEvents("sess")
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ID == "" {
		t.Error("expected auto-assigned ID, got empty string")
	}
}

func TestEmitEvent_SetsCreatedAt(t *testing.T) {
	s := NewInMemoryStore()
	before := time.Now()
	if _, err := s.EmitEvent("sess", session.Event{Role: "user", Content: "hi"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	after := time.Now()

	events, _ := s.GetEvents("sess")
	if events[0].CreatedAt.Before(before) || events[0].CreatedAt.After(after) {
		t.Errorf("CreatedAt %v not in expected range [%v, %v]", events[0].CreatedAt, before, after)
	}
}

func TestEmitEvent_PreservesExistingCreatedAt(t *testing.T) {
	s := NewInMemoryStore()
	fixed := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	e := session.Event{ID: "evt-fixed", Role: "user", Content: "hi", CreatedAt: fixed}

	_, _ = s.EmitEvent("sess", e)
	events, _ := s.GetEvents("sess")
	if !events[0].CreatedAt.Equal(fixed) {
		t.Errorf("expected CreatedAt %v, got %v", fixed, events[0].CreatedAt)
	}
}

func TestEmitEvent_MultipleEvents_OrderPreserved(t *testing.T) {
	s := NewInMemoryStore()
	roles := []string{"user", "assistant", "user"}
	for i, role := range roles {
		_, _ = s.EmitEvent("sess", session.Event{
			ID:      fmt.Sprintf("e%d", i),
			Role:    role,
			Content: fmt.Sprintf("msg-%d", i),
		})
	}
	events, _ := s.GetEvents("sess")
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	for i, ev := range events {
		if ev.Content != fmt.Sprintf("msg-%d", i) {
			t.Errorf("event[%d]: expected msg-%d, got %s", i, i, ev.Content)
		}
	}
}

// ── GetEvents ────────────────────────────────────────────────────────────────

func TestGetEvents_EmptyForUnknownSession(t *testing.T) {
	s := NewInMemoryStore()
	events, err := s.GetEvents("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected empty slice, got %d events", len(events))
	}
}

func TestGetEvents_ReturnsCopy(t *testing.T) {
	s := NewInMemoryStore()
	_, _ = s.EmitEvent("sess", session.Event{ID: "e1", Role: "user", Content: "orig"})

	events, _ := s.GetEvents("sess")
	events[0].Content = "mutated"

	events2, _ := s.GetEvents("sess")
	if events2[0].Content == "mutated" {
		t.Error("GetEvents should return a copy; internal state was mutated")
	}
}

// ── GetSession ───────────────────────────────────────────────────────────────

func TestGetSession_ReturnsErrorForMissingSession(t *testing.T) {
	s := NewInMemoryStore()
	_, _, err := s.GetSession("no-such-session")
	if err == nil {
		t.Fatal("expected error for unknown session, got nil")
	}
}

func TestCreateSession_PersistsProjectID(t *testing.T) {
	s := NewInMemoryStore()
	if err := s.CreateSession(session.Session{ID: "sess-1", ProjectID: "proj-x"}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sess, events, err := s.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess.ProjectID != "proj-x" {
		t.Errorf("expected ProjectID %q, got %q", "proj-x", sess.ProjectID)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestGetSession_ReturnsAllEvents(t *testing.T) {
	s := NewInMemoryStore()
	for i := 0; i < 3; i++ {
		_, _ = s.EmitEvent("sess", session.Event{ID: fmt.Sprintf("e%d", i), Role: "user"})
	}
	_, events, err := s.GetSession("sess")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

// ── ClearSession ─────────────────────────────────────────────────────────────

func TestClearSession_RemovesAllEvents(t *testing.T) {
	s := NewInMemoryStore()
	_, _ = s.EmitEvent("sess", session.Event{ID: "e1", Role: "user", Content: "hi"})
	_, _ = s.EmitEvent("sess", session.Event{ID: "e2", Role: "assistant", Content: "hey"})

	if err := s.ClearSession("sess"); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}
	events, _ := s.GetEvents("sess")
	if len(events) != 0 {
		t.Errorf("expected 0 events after clear, got %d", len(events))
	}
}

func TestClearSession_AllowsReEmitSameID(t *testing.T) {
	s := NewInMemoryStore()
	e := session.Event{ID: "reuse", Role: "user", Content: "first"}
	_, _ = s.EmitEvent("sess", e)
	_ = s.ClearSession("sess")

	e.Content = "second"
	_, _ = s.EmitEvent("sess", e)

	events, _ := s.GetEvents("sess")
	if len(events) != 1 {
		t.Fatalf("expected 1 event after re-emit, got %d", len(events))
	}
	if events[0].Content != "second" {
		t.Errorf("expected content %q after re-emit, got %q", "second", events[0].Content)
	}
}

func TestClearSession_DoesNotAffectOtherSessions(t *testing.T) {
	s := NewInMemoryStore()
	_, _ = s.EmitEvent("sess-a", session.Event{ID: "e1", Role: "user", Content: "a"})
	_, _ = s.EmitEvent("sess-b", session.Event{ID: "e2", Role: "user", Content: "b"})

	_ = s.ClearSession("sess-a")

	eventsB, _ := s.GetEvents("sess-b")
	if len(eventsB) != 1 {
		t.Errorf("clearing sess-a should not affect sess-b; got %d events", len(eventsB))
	}
}

// ── Concurrency ──────────────────────────────────────────────────────────────

func TestEmitEvent_Concurrent_NoPanic(t *testing.T) {
	s := NewInMemoryStore()
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			_, _ = s.EmitEvent("sess", session.Event{
				ID:      fmt.Sprintf("e%d", i),
				Role:    "user",
				Content: fmt.Sprintf("msg-%d", i),
			})
		}(i)
	}
	wg.Wait()

	events, _ := s.GetEvents("sess")
	if len(events) != goroutines {
		t.Errorf("expected %d events, got %d (possible race)", goroutines, len(events))
	}
}
