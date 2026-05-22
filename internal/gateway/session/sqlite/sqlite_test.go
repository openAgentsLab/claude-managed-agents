package sqlite

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"forge/internal/gateway/session"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	return s
}

// ── EmitEvent ────────────────────────────────────────────────────────────────

func TestEmitEvent_IdempotentByID(t *testing.T) {
	s := newTestStore(t)
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
	s := newTestStore(t)
	if _, err := s.EmitEvent("sess", session.Event{Role: "user", Content: "hello"}); err != nil {
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
	s := newTestStore(t)
	before := time.Now().Truncate(time.Second)
	if _, err := s.EmitEvent("sess", session.Event{Role: "user", Content: "hi"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	after := time.Now().Add(time.Second)

	events, _ := s.GetEvents("sess")
	if events[0].CreatedAt.Before(before) || events[0].CreatedAt.After(after) {
		t.Errorf("CreatedAt %v not in expected range [%v, %v]", events[0].CreatedAt, before, after)
	}
}

func TestEmitEvent_PreservesExistingCreatedAt(t *testing.T) {
	s := newTestStore(t)
	fixed := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	e := session.Event{ID: "evt-fixed", Role: "user", Content: "hi", CreatedAt: fixed}

	_, _ = s.EmitEvent("sess", e)
	events, _ := s.GetEvents("sess")
	// SQLite stores Unix seconds — sub-second precision is lost.
	if !events[0].CreatedAt.Equal(fixed) {
		t.Errorf("expected CreatedAt %v, got %v", fixed, events[0].CreatedAt)
	}
}

func TestEmitEvent_MultipleEvents_OrderPreserved(t *testing.T) {
	s := newTestStore(t)
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
	s := newTestStore(t)
	events, err := s.GetEvents("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected empty slice, got %d events", len(events))
	}
}

// ── GetSession ───────────────────────────────────────────────────────────────

func TestGetSession_ReturnsErrorForMissingSession(t *testing.T) {
	s := newTestStore(t)
	_, _, err := s.GetSession("no-such-session")
	if err == nil {
		t.Fatal("expected error for unknown session, got nil")
	}
}

func TestCreateSession_PersistsProjectID(t *testing.T) {
	s := newTestStore(t)
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
	s := newTestStore(t)
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
	s := newTestStore(t)
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
	s := newTestStore(t)
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
	s := newTestStore(t)
	_, _ = s.EmitEvent("sess-a", session.Event{ID: "e1", Role: "user", Content: "a"})
	_, _ = s.EmitEvent("sess-b", session.Event{ID: "e2", Role: "user", Content: "b"})

	_ = s.ClearSession("sess-a")

	eventsB, _ := s.GetEvents("sess-b")
	if len(eventsB) != 1 {
		t.Errorf("clearing sess-a should not affect sess-b; got %d events", len(eventsB))
	}
}

// ── Persistence ──────────────────────────────────────────────────────────────

func TestPersistence_SurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")

	s1, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	_, _ = s1.EmitEvent("sess", session.Event{ID: "e1", Role: "user", Content: "hello"})
	s1.db.Close()

	s2, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	events, err := s2.GetEvents("sess")
	if err != nil {
		t.Fatalf("GetEvents after reopen: %v", err)
	}
	if len(events) != 1 || events[0].Content != "hello" {
		t.Errorf("expected persisted event, got %v", events)
	}
}
