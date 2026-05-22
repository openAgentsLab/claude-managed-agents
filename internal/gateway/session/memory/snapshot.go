package memory

import "forge/internal/gateway/session"

// GetSnapshot returns the cached compacted history for sessionID, or nil if none exists.
func (s *InMemorySessionStore) GetSnapshot(sessionID string) (*session.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := s.snapshots[sessionID]
	if snap == nil {
		return nil, nil
	}
	cp := *snap
	return &cp, nil
}

// SaveSnapshot persists a compacted history snapshot for sessionID.
func (s *InMemorySessionStore) SaveSnapshot(sessionID string, snap *session.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *snap
	s.snapshots[sessionID] = &cp
	return nil
}
