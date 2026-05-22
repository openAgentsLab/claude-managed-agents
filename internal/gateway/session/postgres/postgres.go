// Package postgres implements the PostgreSQL-backed SessionStore driver.
// Import with a blank identifier to register the "postgres" driver:
//
//	import _ "forge/internal/gateway/session/postgres"
//
// Configuration options (via SessionConfig.Options):
//   - dsn: PostgreSQL connection string (required)
package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"forge/internal/config"
	"forge/internal/gateway/session"
)

func init() {
	session.Register(config.SessionDriverPostgres, func(opts map[string]string) (session.SessionStore, error) {
		dsn := opts["dsn"]
		if dsn == "" {
			return nil, fmt.Errorf("postgres session store: dsn is required")
		}
		return NewPostgresStore(dsn)
	})
}

// PostgresStore implements session.SessionStore using PostgreSQL.
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore opens a PostgreSQL connection pool and runs migrations.
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres session store: open: %w", err)
	}
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)
	if err := migrate(db); err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("postgres session store: migrate: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id               TEXT   PRIMARY KEY,
			project_id       TEXT   NOT NULL DEFAULT '',
			memory_store_ids TEXT   NOT NULL DEFAULT '',
			created_at       BIGINT NOT NULL
		);
		ALTER TABLE sessions ADD COLUMN IF NOT EXISTS project_id TEXT NOT NULL DEFAULT '';
		ALTER TABLE sessions ADD COLUMN IF NOT EXISTS memory_store_ids TEXT NOT NULL DEFAULT '';
		ALTER TABLE sessions ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT '';
		ALTER TABLE sessions ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'idle';
		ALTER TABLE sessions ADD COLUMN IF NOT EXISTS custom_tools_json TEXT NOT NULL DEFAULT '';
		ALTER TABLE sessions ADD COLUMN IF NOT EXISTS agent_id TEXT NOT NULL DEFAULT '';
		ALTER TABLE sessions ADD COLUMN IF NOT EXISTS init_error TEXT NOT NULL DEFAULT '';
		ALTER TABLE sessions ADD COLUMN IF NOT EXISTS project_snapshot_json TEXT NOT NULL DEFAULT '';
		CREATE TABLE IF NOT EXISTS events (
			session_id   TEXT    NOT NULL REFERENCES sessions(id),
			id           TEXT    NOT NULL,
			role         TEXT    NOT NULL,
			content      TEXT    NOT NULL DEFAULT '',
			tool_name    TEXT    NOT NULL DEFAULT '',
			tool_call_id TEXT    NOT NULL DEFAULT '',
			pending      BOOLEAN NOT NULL DEFAULT TRUE,
			created_at   BIGINT  NOT NULL,
			seq          BIGINT  NOT NULL DEFAULT 0,
			PRIMARY KEY (session_id, id)
		);
		CREATE SEQUENCE IF NOT EXISTS events_seq_sequence START 1;
		ALTER TABLE events ADD COLUMN IF NOT EXISTS seq BIGINT NOT NULL DEFAULT 0;
		CREATE INDEX IF NOT EXISTS idx_events_session_time ON events(session_id, created_at ASC);
		CREATE INDEX IF NOT EXISTS idx_events_session_seq ON events(session_id, seq ASC);
		CREATE TABLE IF NOT EXISTS snapshots (
			session_id    TEXT    PRIMARY KEY,
			messages_json TEXT    NOT NULL DEFAULT '',
			event_count   INTEGER NOT NULL DEFAULT 0,
			created_at    BIGINT  NOT NULL
		);
	`)
	return err
}

// CreateSession registers a new session.
func (s *PostgresStore) CreateSession(sess session.Session) error {
	idsJSON, err := json.Marshal(sess.MemoryStoreIDs)
	if err != nil {
		return fmt.Errorf("postgres session store: marshal memory_store_ids: %w", err)
	}
	toolsJSON, err := json.Marshal(sess.CustomTools)
	if err != nil {
		return fmt.Errorf("postgres session store: marshal custom_tools: %w", err)
	}
	status := sess.Status
	if status == "" {
		status = session.SessionIdle
	}
	_, err = s.db.Exec(
		`INSERT INTO sessions (id, project_id, agent_id, memory_store_ids, custom_tools_json, status, project_snapshot_json, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT (id) DO NOTHING`,
		sess.ID, sess.ProjectID, sess.AgentID, string(idsJSON), string(toolsJSON), string(status), sess.ProjectSnapshot, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("postgres session store: create session %q: %w", sess.ID, err)
	}
	return nil
}

// GetSession returns session metadata and all events for sessionID.
func (s *PostgresStore) GetSession(sessionID string) (*session.Session, []session.Event, error) {
	sess := &session.Session{ID: sessionID}

	var createdAt int64
	var idsJSON, toolsJSON, status, initError, projectSnapshot string
	err := s.db.QueryRow(`SELECT project_id, agent_id, memory_store_ids, title, status, init_error, custom_tools_json, project_snapshot_json, created_at FROM sessions WHERE id = $1`, sessionID).Scan(&sess.ProjectID, &sess.AgentID, &idsJSON, &sess.Title, &status, &initError, &toolsJSON, &projectSnapshot, &createdAt)
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
		return nil, nil, fmt.Errorf("postgres session store: session %q not found", sessionID)
	default:
		return nil, nil, fmt.Errorf("postgres session store: get session %q: %w", sessionID, err)
	}

	events, err := s.queryEvents(sessionID)
	if err != nil {
		return nil, nil, err
	}
	return sess, events, nil
}

// GetEvents returns all events for sessionID ordered by insertion time.
func (s *PostgresStore) GetEvents(sessionID string) ([]session.Event, error) {
	return s.queryEvents(sessionID)
}

func (s *PostgresStore) queryEvents(sessionID string) ([]session.Event, error) {
	return s.queryEventsSince(sessionID, 0)
}

func (s *PostgresStore) queryEventsSince(sessionID string, afterSeq int64) ([]session.Event, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if afterSeq <= 0 {
		rows, err = s.db.Query(`
			SELECT seq, id, role, content, tool_name, tool_call_id, pending, created_at
			FROM events
			WHERE session_id = $1
			ORDER BY created_at ASC, ctid ASC
		`, sessionID)
	} else {
		rows, err = s.db.Query(`
			SELECT seq, id, role, content, tool_name, tool_call_id, pending, created_at
			FROM events
			WHERE session_id = $1 AND seq > $2
			ORDER BY seq ASC
		`, sessionID, afterSeq)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres session store: query events for %q: %w", sessionID, err)
	}
	defer rows.Close()

	var events []session.Event
	for rows.Next() {
		var e session.Event
		var pending bool
		var createdAt int64
		if err := rows.Scan(&e.Seq, &e.ID, &e.Role, &e.Content, &e.ToolName, &e.ToolCallID, &pending, &createdAt); err != nil {
			return nil, fmt.Errorf("postgres session store: scan event: %w", err)
		}
		e.Pending = pending
		e.CreatedAt = time.Unix(createdAt, 0)
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetEventsSince returns events with seq > afterSeq ordered by seq ASC.
// afterSeq=0 returns all events ordered by created_at/ctid.
func (s *PostgresStore) GetEventsSince(sessionID string, afterSeq int64) ([]session.Event, error) {
	return s.queryEventsSince(sessionID, afterSeq)
}

// EmitEvent idempotently appends an event to the session's event log.
// Repeated calls with the same event.ID are silently ignored (returns seq=0).
// Returns the seq assigned to the event by the global sequence.
func (s *PostgresStore) EmitEvent(sessionID string, e session.Event) (int64, error) {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("postgres session store: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err = tx.Exec(
		`INSERT INTO sessions (id, created_at) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`,
		sessionID, e.CreatedAt.Unix(),
	); err != nil {
		return 0, fmt.Errorf("postgres session store: upsert session: %w", err)
	}

	var seq int64
	err = tx.QueryRow(
		`INSERT INTO events (id, session_id, role, content, tool_name, tool_call_id, pending, created_at, seq)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, nextval('events_seq_sequence'))
		 ON CONFLICT (session_id, id) DO NOTHING
		 RETURNING seq`,
		e.ID, sessionID, e.Role, e.Content, e.ToolName, e.ToolCallID, e.Pending, e.CreatedAt.Unix(),
	).Scan(&seq)
	if err == sql.ErrNoRows {
		err = nil // duplicate — event already exists, seq unknown
	} else if err != nil {
		return 0, fmt.Errorf("postgres session store: insert event: %w", err)
	}

	return seq, tx.Commit()
}

// ClearSession removes all events, the snapshot, and the session record for sessionID.
func (s *PostgresStore) ClearSession(sessionID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("postgres session store: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM events WHERE session_id = $1`, sessionID); err != nil {
		return fmt.Errorf("postgres session store: clear events: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM snapshots WHERE session_id = $1`, sessionID); err != nil {
		return fmt.Errorf("postgres session store: clear snapshot: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE id = $1`, sessionID); err != nil {
		return fmt.Errorf("postgres session store: clear session: %w", err)
	}
	return tx.Commit()
}

// ResetHistory deletes all events and the snapshot for sessionID but keeps the session record.
func (s *PostgresStore) ResetHistory(sessionID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("postgres session store: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`DELETE FROM events WHERE session_id = $1`, sessionID); err != nil {
		return fmt.Errorf("postgres session store: reset events: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM snapshots WHERE session_id = $1`, sessionID); err != nil {
		return fmt.Errorf("postgres session store: reset snapshot: %w", err)
	}
	return tx.Commit()
}

// ListSessions returns sessions whose internal ID begins with userScope+":".
func (s *PostgresStore) ListSessions(userScope string) ([]session.Session, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, title, status, init_error, created_at FROM sessions WHERE id LIKE $1 ORDER BY created_at DESC`,
		userScope+":%",
	)
	if err != nil {
		return nil, fmt.Errorf("postgres session store: list sessions: %w", err)
	}
	defer rows.Close()
	prefix := userScope + ":"
	var out []session.Session
	for rows.Next() {
		var sess session.Session
		var createdAt int64
		var status string
		if err := rows.Scan(&sess.ID, &sess.ProjectID, &sess.Title, &status, &sess.InitError, &createdAt); err != nil {
			return nil, fmt.Errorf("postgres session store: scan session: %w", err)
		}
		sess.ID = strings.TrimPrefix(sess.ID, prefix)
		sess.Status = session.SessionStatus(status)
		sess.CreatedAt = time.Unix(createdAt, 0).UTC()
		out = append(out, sess)
	}
	return out, rows.Err()
}

// ListProjectSessions returns sessions for userScope filtered by projectID at the DB layer.
func (s *PostgresStore) ListProjectSessions(userScope, projectID string) ([]session.Session, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, title, status, init_error, created_at FROM sessions WHERE id LIKE $1 AND project_id = $2 ORDER BY created_at DESC`,
		userScope+":%",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres session store: list project sessions: %w", err)
	}
	defer rows.Close()
	prefix := userScope + ":"
	var out []session.Session
	for rows.Next() {
		var sess session.Session
		var createdAt int64
		var status string
		if err := rows.Scan(&sess.ID, &sess.ProjectID, &sess.Title, &status, &sess.InitError, &createdAt); err != nil {
			return nil, fmt.Errorf("postgres session store: scan session: %w", err)
		}
		sess.ID = strings.TrimPrefix(sess.ID, prefix)
		sess.Status = session.SessionStatus(status)
		sess.CreatedAt = time.Unix(createdAt, 0).UTC()
		out = append(out, sess)
	}
	return out, rows.Err()
}

// UpdateSessionTitle sets the display title for a session.
func (s *PostgresStore) UpdateSessionTitle(sessionID, title string) error {
	_, err := s.db.Exec(`UPDATE sessions SET title = $1 WHERE id = $2`, title, sessionID)
	if err != nil {
		return fmt.Errorf("postgres session store: update title for %q: %w", sessionID, err)
	}
	return nil
}

// UpdateCustomTools replaces the custom tool definitions for a session.
func (s *PostgresStore) UpdateCustomTools(sessionID string, tools []session.CustomToolDef) error {
	b, err := json.Marshal(tools)
	if err != nil {
		return fmt.Errorf("postgres session store: marshal custom_tools: %w", err)
	}
	_, err = s.db.Exec(`UPDATE sessions SET custom_tools_json = $1 WHERE id = $2`, string(b), sessionID)
	if err != nil {
		return fmt.Errorf("postgres session store: update custom_tools for %q: %w", sessionID, err)
	}
	return nil
}

// UpdateSessionStatus transitions the session to the given status.
func (s *PostgresStore) UpdateSessionStatus(sessionID string, status session.SessionStatus) error {
	_, err := s.db.Exec(`UPDATE sessions SET status = $1 WHERE id = $2`, string(status), sessionID)
	if err != nil {
		return fmt.Errorf("postgres session store: update status for %q: %w", sessionID, err)
	}
	return nil
}

// UpdateSessionInitStatus transitions a session from initializing to idle or init_failed.
func (s *PostgresStore) UpdateSessionInitStatus(sessionID string, status session.SessionStatus, initError string) error {
	_, err := s.db.Exec(`UPDATE sessions SET status = $1, init_error = $2 WHERE id = $3`, string(status), initError, sessionID)
	if err != nil {
		return fmt.Errorf("postgres session store: update init status for %q: %w", sessionID, err)
	}
	return nil
}

// HasSessionsForProject reports whether any sessions reference the given project ID.
func (s *PostgresStore) HasSessionsForProject(projectID string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM sessions WHERE project_id = $1 LIMIT 1`, projectID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("postgres session store: has sessions for project %q: %w", projectID, err)
	}
	return count > 0, nil
}

// GetSnapshot returns the cached compacted history for sessionID, or (nil, nil) if none exists.
func (s *PostgresStore) GetSnapshot(sessionID string) (*session.Snapshot, error) {
	var messagesJSON string
	var eventCount int
	var createdAt int64

	err := s.db.QueryRow(
		`SELECT messages_json, event_count, created_at FROM snapshots WHERE session_id = $1`,
		sessionID,
	).Scan(&messagesJSON, &eventCount, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres session store: get snapshot %q: %w", sessionID, err)
	}

	var msgs []*schema.Message
	if err := json.Unmarshal([]byte(messagesJSON), &msgs); err != nil {
		return nil, fmt.Errorf("postgres session store: unmarshal snapshot messages: %w", err)
	}
	return &session.Snapshot{
		Messages:   msgs,
		EventCount: eventCount,
		CreatedAt:  time.Unix(createdAt, 0),
	}, nil
}

// SaveSnapshot persists a compacted history snapshot for sessionID.
func (s *PostgresStore) SaveSnapshot(sessionID string, snap *session.Snapshot) error {
	b, err := json.Marshal(snap.Messages)
	if err != nil {
		return fmt.Errorf("postgres session store: marshal snapshot messages: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO snapshots (session_id, messages_json, event_count, created_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (session_id) DO UPDATE SET messages_json=$2, event_count=$3, created_at=$4`,
		sessionID, string(b), snap.EventCount, snap.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("postgres session store: save snapshot %q: %w", sessionID, err)
	}
	return nil
}
