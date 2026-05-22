package postgres

import (
	"context"
	"database/sql"
	"time"

	"forge/internal/gateway/store"
)

// ── SandboxRepository ──────────────────────────────────────────────────────

type sandboxRepo struct{ db *sql.DB }

func (r *sandboxRepo) Upsert(_ context.Context, rec store.SandboxRecord) error {
	_, err := r.db.Exec(
		`INSERT INTO sandboxes (session_id, tenant_id, sandbox_id, endpoint, token, last_seen, environment_spec)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (session_id) DO UPDATE SET
		   tenant_id        = EXCLUDED.tenant_id,
		   sandbox_id       = EXCLUDED.sandbox_id,
		   endpoint         = EXCLUDED.endpoint,
		   token            = EXCLUDED.token,
		   last_seen        = EXCLUDED.last_seen,
		   environment_spec = EXCLUDED.environment_spec`,
		rec.SessionID, rec.TenantID, rec.SandboxID, rec.Endpoint, rec.Token, rec.LastSeen, rec.EnvironmentSpec,
	)
	return wrapErr("upsert sandbox", err)
}

func (r *sandboxRepo) Get(_ context.Context, sessionID string) (*store.SandboxRecord, error) {
	var rec store.SandboxRecord
	err := r.db.QueryRow(
		`SELECT session_id, tenant_id, sandbox_id, endpoint, token, last_seen, environment_spec
		 FROM sandboxes WHERE session_id = $1`,
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
		 VALUES ($1, $2)
		 ON CONFLICT (session_id) DO UPDATE SET environment_spec = EXCLUDED.environment_spec`,
		sessionID, spec,
	)
	return wrapErr("set environment spec", err)
}

func (r *sandboxRepo) Delete(_ context.Context, sessionID string) error {
	_, err := r.db.Exec(`DELETE FROM sandboxes WHERE session_id = $1`, sessionID)
	return wrapErr("delete sandbox", err)
}

func (r *sandboxRepo) Touch(_ context.Context, sessionID string) error {
	_, err := r.db.Exec(
		`UPDATE sandboxes SET last_seen = $1 WHERE session_id = $2`,
		time.Now().Unix(), sessionID,
	)
	return wrapErr("touch sandbox", err)
}

func (r *sandboxRepo) DeleteStaleBefore(_ context.Context, cutoff time.Time) (int, error) {
	res, err := r.db.Exec(
		`DELETE FROM sandboxes WHERE last_seen < $1`, cutoff.Unix(),
	)
	if err != nil {
		return 0, wrapErr("delete stale sandboxes", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ── SessionResourceRepository ─────────────────────────────────────────────

type sessionResourceRepo struct{ db *sql.DB }

func (r *sessionResourceRepo) Upsert(_ context.Context, rec store.SessionResourceRecord) error {
	_, err := r.db.Exec(
		`INSERT INTO session_resources (id, session_id, type, target_path, spec, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO UPDATE SET
		   session_id  = EXCLUDED.session_id,
		   type        = EXCLUDED.type,
		   target_path = EXCLUDED.target_path,
		   spec        = EXCLUDED.spec`,
		rec.ID, rec.SessionID, rec.Type, rec.TargetPath, rec.Spec, rec.CreatedAt,
	)
	return wrapErr("upsert session resource", err)
}

func (r *sessionResourceRepo) List(_ context.Context, sessionID string) ([]store.SessionResourceRecord, error) {
	rows, err := r.db.Query(
		`SELECT id, session_id, type, target_path, spec, created_at
		 FROM session_resources WHERE session_id = $1 ORDER BY created_at`,
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
	_, err := r.db.Exec(`DELETE FROM session_resources WHERE id = $1`, id)
	return wrapErr("delete session resource", err)
}

func (r *sessionResourceRepo) DeleteBySession(_ context.Context, sessionID string) error {
	_, err := r.db.Exec(`DELETE FROM session_resources WHERE session_id = $1`, sessionID)
	return wrapErr("delete session resources by session", err)
}
