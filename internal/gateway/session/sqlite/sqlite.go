// Package sqlite implements the SQLite-backed SessionStore driver.
// Import with a blank identifier to register the "sqlite" driver:
//
//	import _ "forge/internal/gateway/session/sqlite"
//
// Configuration options (via SessionConfig.Options):
//   - path: path to the SQLite database file (default "forge.db")
package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"forge/internal/config"
	"forge/internal/gateway/session"
)

const defaultPath = "forge.db"

func init() {
	session.Register(config.SessionDriverSQLite, func(opts map[string]string) (session.SessionStore, error) {
		path := opts["path"]
		if path == "" {
			path = defaultPath
		}
		return NewSQLiteStore(path)
	})
}

// SQLiteStore implements session.SessionStore using a local SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at path and runs migrations.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: open %q: %w", path, err)
	}
	// SQLite supports only one writer at a time; cap pool to 1.
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("sqlite store: migrate: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id         TEXT    PRIMARY KEY,
			project_id TEXT    NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS events (
			id           TEXT    NOT NULL,
			session_id   TEXT    NOT NULL REFERENCES sessions(id),
			role         TEXT    NOT NULL,
			content      TEXT    NOT NULL DEFAULT '',
			tool_name    TEXT    NOT NULL DEFAULT '',
			tool_call_id TEXT    NOT NULL DEFAULT '',
			pending      INTEGER NOT NULL DEFAULT 1,
			created_at   INTEGER NOT NULL,
			PRIMARY KEY (session_id, id)
		);
		CREATE TABLE IF NOT EXISTS snapshots (
			session_id   TEXT    PRIMARY KEY,
			messages_json TEXT   NOT NULL DEFAULT '',
			event_count  INTEGER NOT NULL DEFAULT 0,
			created_at   INTEGER NOT NULL
		);
	`)
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE events ADD COLUMN tool_call_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN project_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN memory_store_ids TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN title TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN status TEXT NOT NULL DEFAULT 'idle'`,
		`ALTER TABLE sessions ADD COLUMN custom_tools_json TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN agent_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN init_error TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE sessions ADD COLUMN project_snapshot_json TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err = db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}

// CreateSession registers a new session.
func (s *SQLiteStore) CreateSession(sess session.Session) error {
	idsJSON, err := json.Marshal(sess.MemoryStoreIDs)
	if err != nil {
		return fmt.Errorf("sqlite store: marshal memory_store_ids: %w", err)
	}
	toolsJSON, err := json.Marshal(sess.CustomTools)
	if err != nil {
		return fmt.Errorf("sqlite store: marshal custom_tools: %w", err)
	}
	status := sess.Status
	if status == "" {
		status = session.SessionIdle
	}
	_, err = s.db.Exec(
		`INSERT OR IGNORE INTO sessions (id, project_id, agent_id, memory_store_ids, custom_tools_json, status, project_snapshot_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.ProjectID, sess.AgentID, string(idsJSON), string(toolsJSON), string(status), sess.ProjectSnapshot, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("sqlite store: create session %q: %w", sess.ID, err)
	}
	return nil
}

// GetSession returns session metadata and all events for sessionID.
func (s *SQLiteStore) GetSession(sessionID string) (*session.Session, []session.Event, error) {
	sess := &session.Session{ID: sessionID}

	var createdAt int64
	var idsJSON, toolsJSON, status, initError, projectSnapshot string
	err := s.db.QueryRow(`SELECT project_id, agent_id, memory_store_ids, title, status, init_error, custom_tools_json, project_snapshot_json, created_at FROM sessions WHERE id = ?`, sessionID).Scan(&sess.ProjectID, &sess.AgentID, &idsJSON, &sess.Title, &status, &initError, &toolsJSON, &projectSnapshot, &createdAt)
	switch err {
	case nil:
		sess.CreatedAt = time.Unix(createdAt, 0)
		sess.Status = session.SessionStatus(status)
		sess.InitError = initError
		sess.ProjectSnapshot = projectSnapshot
		if idsJSON != "" && idsJSON != "null" {
			_ = json.Unmarshal([]byte(idsJSON), &sess.MemoryStoreIDs)
		}
		if toolsJSON != "" && toolsJSON != "null" {
			_ = json.Unmarshal([]byte(toolsJSON), &sess.CustomTools)
		}
	case sql.ErrNoRows:
		return nil, nil, fmt.Errorf("sqlite store: session %q not found", sessionID)
	default:
		return nil, nil, fmt.Errorf("sqlite store: get session %q: %w", sessionID, err)
	}

	events, err := s.queryEvents(sessionID)
	if err != nil {
		return nil, nil, err
	}
	return sess, events, nil
}

// GetEvents returns all events for sessionID ordered by insertion time.
func (s *SQLiteStore) GetEvents(sessionID string) ([]session.Event, error) {
	return s.queryEvents(sessionID)
}

func (s *SQLiteStore) queryEvents(sessionID string) ([]session.Event, error) {
	return s.queryEventsSince(sessionID, -1) // -1 = all events
}

func (s *SQLiteStore) queryEventsSince(sessionID string, afterRowid int64) ([]session.Event, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if afterRowid <= 0 {
		rows, err = s.db.Query(`
			SELECT rowid, id, role, content, tool_name, tool_call_id, pending, created_at
			FROM events
			WHERE session_id = ?
			ORDER BY created_at ASC, rowid ASC
		`, sessionID)
	} else {
		rows, err = s.db.Query(`
			SELECT rowid, id, role, content, tool_name, tool_call_id, pending, created_at
			FROM events
			WHERE session_id = ? AND rowid > ?
			ORDER BY rowid ASC
		`, sessionID, afterRowid)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite store: query events for %q: %w", sessionID, err)
	}
	defer rows.Close()

	var events []session.Event
	for rows.Next() {
		var e session.Event
		var pending int
		var createdAt int64
		if err := rows.Scan(&e.Seq, &e.ID, &e.Role, &e.Content, &e.ToolName, &e.ToolCallID, &pending, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite store: scan event: %w", err)
		}
		e.Pending = pending == 1
		e.CreatedAt = time.Unix(createdAt, 0)
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetEventsSince returns events with rowid > afterSeq, ordered by rowid ASC.
// afterSeq=0 returns all events (ordered by created_at, rowid for stable sort).
func (s *SQLiteStore) GetEventsSince(sessionID string, afterSeq int64) ([]session.Event, error) {
	return s.queryEventsSince(sessionID, afterSeq)
}

// EmitEvent idempotently appends an event to the session's event log.
// Repeated calls with the same event.ID are silently ignored (returns seq=0).
// Returns the rowid (Seq) assigned to the event.
func (s *SQLiteStore) EmitEvent(sessionID string, e session.Event) (int64, error) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("sqlite store: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err = tx.Exec(
		`INSERT OR IGNORE INTO sessions (id, created_at) VALUES (?, ?)`,
		sessionID, e.CreatedAt.Unix(),
	); err != nil {
		return 0, fmt.Errorf("sqlite store: upsert session: %w", err)
	}

	pending := 0
	if e.Pending {
		pending = 1
	}
	var seq int64
	err = tx.QueryRow(
		`INSERT OR IGNORE INTO events (id, session_id, role, content, tool_name, tool_call_id, pending, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 RETURNING rowid`,
		e.ID, sessionID, e.Role, e.Content, e.ToolName, e.ToolCallID, pending, e.CreatedAt.Unix(),
	).Scan(&seq)
	if err == sql.ErrNoRows {
		err = nil // duplicate — event already exists, seq unknown
	} else if err != nil {
		return 0, fmt.Errorf("sqlite store: insert event: %w", err)
	}

	return seq, tx.Commit()
}

// ClearSession removes all events, the snapshot, and the session record for sessionID.
func (s *SQLiteStore) ClearSession(sessionID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("sqlite store: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM events WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("sqlite store: clear events: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM snapshots WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("sqlite store: clear snapshot: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, sessionID); err != nil {
		return fmt.Errorf("sqlite store: clear session: %w", err)
	}
	return tx.Commit()
}

// ResetHistory deletes all events and the snapshot for sessionID but keeps the session record.
func (s *SQLiteStore) ResetHistory(sessionID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("sqlite store: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM events WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("sqlite store: reset events: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM snapshots WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("sqlite store: reset snapshot: %w", err)
	}
	return tx.Commit()
}

// ListSessions returns sessions whose internal ID begins with userScope+":".
func (s *SQLiteStore) ListSessions(userScope string) ([]session.Session, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, title, status, init_error, created_at FROM sessions WHERE id LIKE ? ORDER BY created_at DESC`,
		userScope+":%",
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: list sessions: %w", err)
	}
	defer rows.Close()
	prefix := userScope + ":"
	var out []session.Session
	for rows.Next() {
		var sess session.Session
		var createdAt int64
		var status string
		if err := rows.Scan(&sess.ID, &sess.ProjectID, &sess.Title, &status, &sess.InitError, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite store: scan session: %w", err)
		}
		sess.ID = strings.TrimPrefix(sess.ID, prefix)
		sess.Status = session.SessionStatus(status)
		sess.CreatedAt = time.Unix(createdAt, 0).UTC()
		out = append(out, sess)
	}
	return out, rows.Err()
}

// ListProjectSessions returns sessions for userScope filtered by projectID at the DB layer.
func (s *SQLiteStore) ListProjectSessions(userScope, projectID string) ([]session.Session, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, title, status, init_error, created_at FROM sessions WHERE id LIKE ? AND project_id = ? ORDER BY created_at DESC`,
		userScope+":%",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: list project sessions: %w", err)
	}
	defer rows.Close()
	prefix := userScope + ":"
	var out []session.Session
	for rows.Next() {
		var sess session.Session
		var createdAt int64
		var status string
		if err := rows.Scan(&sess.ID, &sess.ProjectID, &sess.Title, &status, &sess.InitError, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite store: scan session: %w", err)
		}
		sess.ID = strings.TrimPrefix(sess.ID, prefix)
		sess.Status = session.SessionStatus(status)
		sess.CreatedAt = time.Unix(createdAt, 0).UTC()
		out = append(out, sess)
	}
	return out, rows.Err()
}

// UpdateSessionTitle sets the display title for a session.
func (s *SQLiteStore) UpdateSessionTitle(sessionID, title string) error {
	_, err := s.db.Exec(`UPDATE sessions SET title = ? WHERE id = ?`, title, sessionID)
	if err != nil {
		return fmt.Errorf("sqlite store: update title for %q: %w", sessionID, err)
	}
	return nil
}

// UpdateCustomTools replaces the custom tool definitions for a session.
func (s *SQLiteStore) UpdateCustomTools(sessionID string, tools []session.CustomToolDef) error {
	b, err := json.Marshal(tools)
	if err != nil {
		return fmt.Errorf("sqlite store: marshal custom_tools: %w", err)
	}
	_, err = s.db.Exec(`UPDATE sessions SET custom_tools_json = ? WHERE id = ?`, string(b), sessionID)
	if err != nil {
		return fmt.Errorf("sqlite store: update custom_tools for %q: %w", sessionID, err)
	}
	return nil
}

// UpdateSessionStatus transitions the session to the given status.
func (s *SQLiteStore) UpdateSessionStatus(sessionID string, status session.SessionStatus) error {
	_, err := s.db.Exec(`UPDATE sessions SET status = ? WHERE id = ?`, string(status), sessionID)
	if err != nil {
		return fmt.Errorf("sqlite store: update status for %q: %w", sessionID, err)
	}
	return nil
}

// UpdateSessionInitStatus transitions a session from initializing to idle or init_failed.
func (s *SQLiteStore) UpdateSessionInitStatus(sessionID string, status session.SessionStatus, initError string) error {
	_, err := s.db.Exec(`UPDATE sessions SET status = ?, init_error = ? WHERE id = ?`, string(status), initError, sessionID)
	if err != nil {
		return fmt.Errorf("sqlite store: update init status for %q: %w", sessionID, err)
	}
	return nil
}

// HasSessionsForProject reports whether any sessions reference the given project ID.
func (s *SQLiteStore) HasSessionsForProject(projectID string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM sessions WHERE project_id = ? LIMIT 1`, projectID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("sqlite store: has sessions for project %q: %w", projectID, err)
	}
	return count > 0, nil
}

// GetSnapshot returns the cached compacted history for sessionID, or (nil, nil) if none exists.
func (s *SQLiteStore) GetSnapshot(sessionID string) (*session.Snapshot, error) {
	var messagesJSON string
	var eventCount int
	var createdAt int64

	err := s.db.QueryRow(
		`SELECT messages_json, event_count, created_at FROM snapshots WHERE session_id = ?`,
		sessionID,
	).Scan(&messagesJSON, &eventCount, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite store: get snapshot %q: %w", sessionID, err)
	}

	var msgs []*schema.Message
	if err := json.Unmarshal([]byte(messagesJSON), &msgs); err != nil {
		return nil, fmt.Errorf("sqlite store: unmarshal snapshot messages: %w", err)
	}
	return &session.Snapshot{
		Messages:   msgs,
		EventCount: eventCount,
		CreatedAt:  time.Unix(createdAt, 0),
	}, nil
}

// SaveSnapshot persists a compacted history snapshot for sessionID.
func (s *SQLiteStore) SaveSnapshot(sessionID string, snap *session.Snapshot) error {
	b, err := json.Marshal(snap.Messages)
	if err != nil {
		return fmt.Errorf("sqlite store: marshal snapshot messages: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO snapshots (session_id, messages_json, event_count, created_at)
		 VALUES (?, ?, ?, ?)`,
		sessionID, string(b), snap.EventCount, snap.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("sqlite store: save snapshot %q: %w", sessionID, err)
	}
	return nil
}
