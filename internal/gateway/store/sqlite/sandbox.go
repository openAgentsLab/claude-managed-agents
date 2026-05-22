package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"forge/internal/gateway/store"
)

func migrateSandboxes(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sandboxes (
			session_id        TEXT    PRIMARY KEY,
			sandbox_id        TEXT    NOT NULL DEFAULT '',
			endpoint          TEXT    NOT NULL DEFAULT '',
			token             TEXT    NOT NULL DEFAULT '',
			last_seen         INTEGER NOT NULL DEFAULT 0,
			environment_spec  TEXT    NOT NULL DEFAULT ''
		)`)
	if err != nil {
		return err
	}
	// Idempotent migrations for older schemas.
	_, _ = db.Exec(`ALTER TABLE sandboxes RENAME COLUMN user_id TO session_id`)
	_, _ = db.Exec(`ALTER TABLE sandboxes ADD COLUMN packages_spec TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE sandboxes ADD COLUMN networking_spec TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE sandboxes ADD COLUMN environment_spec TEXT NOT NULL DEFAULT ''`)
	if _, err := db.Exec(`ALTER TABLE sandboxes ADD COLUMN tenant_id TEXT NOT NULL DEFAULT ''`); err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("migrate sandboxes.tenant_id: %w", err)
	}
	return nil
}

func migrateSessionResources(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS session_resources (
			id          TEXT    PRIMARY KEY,
			session_id  TEXT    NOT NULL,
			type        TEXT    NOT NULL,
			target_path TEXT    NOT NULL,
			spec        TEXT    NOT NULL DEFAULT '',
			created_at  INTEGER NOT NULL DEFAULT 0
		)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_session_resources_session_id ON session_resources(session_id)`)
	return err
}

// ── SandboxRepository ──────────────────────────────────────────────────────────

type sandboxRepo struct{ db *sql.DB }

func (r *sandboxRepo) Upsert(_ context.Context, rec store.SandboxRecord) error {
	_, err := r.db.Exec(
		`INSERT INTO sandboxes (session_id, tenant_id, sandbox_id, endpoint, token, last_seen, environment_spec)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(session_id) DO UPDATE SET
		   tenant_id        = excluded.tenant_id,
		   sandbox_id       = excluded.sandbox_id,
		   endpoint         = excluded.endpoint,
		   token            = excluded.token,
		   last_seen        = excluded.last_seen,
		   environment_spec = excluded.environment_spec`,
		rec.SessionID, rec.TenantID, rec.SandboxID, rec.Endpoint, rec.Token, rec.LastSeen, rec.EnvironmentSpec,
	)
	return wrapErr("upsert sandbox", err)
}

func (r *sandboxRepo) Get(_ context.Context, sessionID string) (*store.SandboxRecord, error) {
	var rec store.SandboxRecord
	err := r.db.QueryRow(
		`SELECT session_id, tenant_id, sandbox_id, endpoint, token, last_seen, environment_spec
		 FROM sandboxes WHERE session_id = ?`,
		sessionID,
	).Scan(&rec.SessionID, &rec.TenantID, &rec.SandboxID, &rec.Endpoint, &rec.Token, &rec.LastSeen, &rec.EnvironmentSpec)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, wrapErr("get sandbox", err)
	}
	return &rec, nil
}

func (r *sandboxRepo) SetEnvironmentSpec(_ context.Context, sessionID, spec string) error {
	_, err := r.db.Exec(
		`INSERT INTO sandboxes (session_id, environment_spec)
		 VALUES (?, ?)
		 ON CONFLICT(session_id) DO UPDATE SET environment_spec = excluded.environment_spec`,
		sessionID, spec,
	)
	return wrapErr("set environment spec", err)
}

func (r *sandboxRepo) Delete(_ context.Context, sessionID string) error {
	_, err := r.db.Exec(`DELETE FROM sandboxes WHERE session_id = ?`, sessionID)
	return wrapErr("delete sandbox", err)
}

func (r *sandboxRepo) Touch(_ context.Context, sessionID string) error {
	_, err := r.db.Exec(
		`UPDATE sandboxes SET last_seen = ? WHERE session_id = ?`,
		time.Now().Unix(), sessionID,
	)
	return wrapErr("touch sandbox", err)
}

func (r *sandboxRepo) DeleteStaleBefore(_ context.Context, cutoff time.Time) (int, error) {
	res, err := r.db.Exec(
		`DELETE FROM sandboxes WHERE last_seen < ?`, cutoff.Unix(),
	)
	if err != nil {
		return 0, wrapErr("delete stale sandboxes", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ── SessionResourceRepository ─────────────────────────────────────────────────

type sessionResourceRepo struct{ db *sql.DB }

func (r *sessionResourceRepo) Upsert(_ context.Context, rec store.SessionResourceRecord) error {
	_, err := r.db.Exec(
		`INSERT INTO session_resources (id, session_id, type, target_path, spec, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   session_id  = excluded.session_id,
		   type        = excluded.type,
		   target_path = excluded.target_path,
		   spec        = excluded.spec`,
		rec.ID, rec.SessionID, rec.Type, rec.TargetPath, rec.Spec, rec.CreatedAt,
	)
	return wrapErr("upsert session resource", err)
}

func (r *sessionResourceRepo) List(_ context.Context, sessionID string) ([]store.SessionResourceRecord, error) {
	rows, err := r.db.Query(
		`SELECT id, session_id, type, target_path, spec, created_at
		 FROM session_resources WHERE session_id = ? ORDER BY created_at`,
		sessionID,
	)
	if err != nil {
		return nil, wrapErr("list session resources", err)
	}
	defer rows.Close()

	var records []store.SessionResourceRecord
	for rows.Next() {
		var rec store.SessionResourceRecord
		if err := rows.Scan(&rec.ID, &rec.SessionID, &rec.Type, &rec.TargetPath, &rec.Spec, &rec.CreatedAt); err != nil {
			return nil, wrapErr("scan session resource", err)
		}
		records = append(records, rec)
	}
	return records, wrapErr("list session resources rows", rows.Err())
}

func (r *sessionResourceRepo) Delete(_ context.Context, id string) error {
	_, err := r.db.Exec(`DELETE FROM session_resources WHERE id = ?`, id)
	return wrapErr("delete session resource", err)
}

func (r *sessionResourceRepo) DeleteBySession(_ context.Context, sessionID string) error {
	_, err := r.db.Exec(`DELETE FROM session_resources WHERE session_id = ?`, sessionID)
	return wrapErr("delete session resources by session", err)
}
