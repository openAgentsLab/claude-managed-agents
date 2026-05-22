// Package memory implements the in-memory SessionStore driver.
// Import with a blank identifier to register the "memory" driver:
//
//	import _ "forge/internal/gateway/session/memory"
package memory

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"forge/internal/config"
	"forge/internal/gateway/session"
)

func init() {
	session.Register(config.SessionDriverMemory, func(_ map[string]string) (session.SessionStore, error) {
		return NewInMemoryStore(), nil
	})
}

// InMemorySessionStore implements session.SessionStore using in-process maps.
// Suitable for Phase 1; replace with SQLiteSessionStore for persistence.
type InMemorySessionStore struct {
	mu         sync.RWMutex
	sessions   map[string]*session.Session
	events     map[string][]session.Event
	seen       map[string]bool // sessionID+eventID → idempotency set
	snapshots  map[string]*session.Snapshot
	seqCounter int64 // atomic monotonic counter for Event.Seq
}

// NewInMemoryStore creates a new empty InMemorySessionStore.
func NewInMemoryStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions:  make(map[string]*session.Session),
		events:    make(map[string][]session.Event),
		seen:      make(map[string]bool),
		snapshots: make(map[string]*session.Snapshot),
	}
}

// CreateSession registers a session with its project scope.
func (s *InMemorySessionStore) CreateSession(sess session.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[sess.ID]; !ok {
		sess.Status = session.SessionIdle
		sess.CreatedAt = time.Now()
		s.sessions[sess.ID] = &sess
	}
	return nil
}

// GetSession returns session metadata and all events.
func (s *InMemorySessionStore) GetSession(sessionID string) (*session.Session, []session.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess := s.sessions[sessionID]
	if sess == nil {
		return nil, nil, fmt.Errorf("session %q not found", sessionID)
	}
	return sess, append([]session.Event(nil), s.events[sessionID]...), nil
}

// GetEvents returns all events for the session.
func (s *InMemorySessionStore) GetEvents(sessionID string) ([]session.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]session.Event(nil), s.events[sessionID]...), nil
}

// EmitEvent idempotently appends an event. Repeated calls with the same
// event.ID are silently ignored (returns seq=0).
// Returns the monotonic sequence number assigned to the event.
func (s *InMemorySessionStore) EmitEvent(sessionID string, e session.Event) (int64, error) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	key := sessionID + ":" + e.ID
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[key] {
		return 0, nil
	}
	s.seen[key] = true
	if s.sessions[sessionID] == nil {
		s.sessions[sessionID] = &session.Session{ID: sessionID, Status: session.SessionIdle, CreatedAt: time.Now()}
	}
	e.Seq = atomic.AddInt64(&s.seqCounter, 1)
	s.events[sessionID] = append(s.events[sessionID], e)
	return e.Seq, nil
}

// UpdateSessionStatus transitions the session to the given status.
func (s *InMemorySessionStore) UpdateSessionStatus(sessionID string, status session.SessionStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess := s.sessions[sessionID]; sess != nil {
		sess.Status = status
	}
	return nil
}

// UpdateSessionInitStatus transitions a session from initializing to idle or init_failed.
func (s *InMemorySessionStore) UpdateSessionInitStatus(sessionID string, status session.SessionStatus, initError string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess := s.sessions[sessionID]; sess != nil {
		sess.Status = status
		sess.InitError = initError
	}
	return nil
}

// HasSessionsForProject reports whether any sessions reference the given project ID.
func (s *InMemorySessionStore) HasSessionsForProject(projectID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sess := range s.sessions {
		if sess.ProjectID == projectID {
			return true, nil
		}
	}
	return false, nil
}

// GetEventsSince returns events with Seq > afterSeq ordered by Seq ASC.
// afterSeq=0 returns all events in insertion order.
func (s *InMemorySessionStore) GetEventsSince(sessionID string, afterSeq int64) ([]session.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := s.events[sessionID]
	if afterSeq <= 0 {
		return append([]session.Event(nil), all...), nil
	}
	var out []session.Event
	for _, e := range all {
		if e.Seq > afterSeq {
			out = append(out, e)
		}
	}
	return out, nil
}

// ListSessions returns sessions whose internal ID begins with userScope+":".
func (s *InMemorySessionStore) ListSessions(userScope string) ([]session.Session, error) {
	prefix := userScope + ":"
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []session.Session
	for id, sess := range s.sessions {
		if strings.HasPrefix(id, prefix) {
			stripped := *sess
			stripped.ID = strings.TrimPrefix(id, prefix)
			out = append(out, stripped)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// ListProjectSessions returns sessions for userScope filtered by projectID.
func (s *InMemorySessionStore) ListProjectSessions(userScope, projectID string) ([]session.Session, error) {
	prefix := userScope + ":"
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []session.Session
	for id, sess := range s.sessions {
		if strings.HasPrefix(id, prefix) && sess.ProjectID == projectID {
			stripped := *sess
			stripped.ID = strings.TrimPrefix(id, prefix)
			out = append(out, stripped)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// ClearSession removes all events, the snapshot, and the session record.
func (s *InMemorySessionStore) ClearSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.events, sessionID)
	delete(s.sessions, sessionID)
	delete(s.snapshots, sessionID)
	for k := range s.seen {
		if len(k) > len(sessionID) && k[:len(sessionID)+1] == sessionID+":" {
			delete(s.seen, k)
		}
	}
	return nil
}

// ResetHistory deletes all events and the snapshot but keeps the session record.
func (s *InMemorySessionStore) ResetHistory(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.events, sessionID)
	delete(s.snapshots, sessionID)
	for k := range s.seen {
		if len(k) > len(sessionID) && k[:len(sessionID)+1] == sessionID+":" {
			delete(s.seen, k)
		}
	}
	return nil
}

// UpdateSessionTitle sets the display title for a session.
func (s *InMemorySessionStore) UpdateSessionTitle(sessionID, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess := s.sessions[sessionID]; sess != nil {
		sess.Title = title
	}
	return nil
}

// UpdateCustomTools replaces the custom tool definitions for a session.
func (s *InMemorySessionStore) UpdateCustomTools(sessionID string, tools []session.CustomToolDef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess := s.sessions[sessionID]; sess != nil {
		sess.CustomTools = tools
	}
	return nil
}
