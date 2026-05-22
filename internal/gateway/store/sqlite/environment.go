package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"forge/internal/gateway/store"
	"forge/internal/gateway/store/encoding"
)

func migrateEnvironments(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS environments (
			id              TEXT    PRIMARY KEY,
			tenant_id       TEXT    NOT NULL,
			owner_id        TEXT    NOT NULL DEFAULT '',
			scope           TEXT    NOT NULL DEFAULT 'tenant',
			name            TEXT    NOT NULL DEFAULT '',
			description     TEXT    NOT NULL DEFAULT '',
			packages_json   TEXT    NOT NULL DEFAULT '{}',
			networking_json TEXT    NOT NULL DEFAULT '{}',
			is_default      INTEGER NOT NULL DEFAULT 0,
			created_at      INTEGER NOT NULL,
			updated_at      INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_environments_scope
			ON environments(tenant_id, scope, owner_id);
	`); err != nil {
		return err
	}
	if _, err := db.Exec(
		`ALTER TABLE environments ADD COLUMN env_json TEXT NOT NULL DEFAULT '{}'`,
	); err != nil && !isDuplicateColumn(err) {
		return fmt.Errorf("migrate environments.env_json: %w", err)
	}
	return nil
}

type environmentRepo struct{ db *sql.DB }

func (r *environmentRepo) Create(_ context.Context, e *store.Environment) error {
	pkgJSON, netJSON, envJSON, err := marshalEnvFields(e)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = r.db.Exec(
		`INSERT INTO environments
			(id, tenant_id, owner_id, scope, name, description, packages_json, networking_json, env_json, created_at, updated_at)
		 VALUES (?, ?, '', 'tenant', ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.TenantID, e.Name, e.Description,
		pkgJSON, netJSON, envJSON, now, now,
	)
	return wrapErr("create environment", err)
}

func (r *environmentRepo) Get(_ context.Context, id string) (*store.Environment, error) {
	row := r.db.QueryRow(
		`SELECT id, tenant_id, owner_id, scope, name, description, packages_json, networking_json, env_json, created_at, updated_at
		 FROM environments WHERE id = ?`, id,
	)
	return scanEnv(row)
}

func (r *environmentRepo) List(_ context.Context, tenantID string) ([]*store.Environment, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, owner_id, scope, name, description, packages_json, networking_json, env_json, created_at, updated_at
		 FROM environments
		 WHERE tenant_id = ? AND scope = 'tenant'
		 ORDER BY created_at ASC`,
		tenantID,
	)
	if err != nil {
		return nil, wrapErr("list environments", err)
	}
	defer rows.Close()

	var out []*store.Environment
	for rows.Next() {
		e, err := scanEnv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *environmentRepo) Update(_ context.Context, e *store.Environment) error {
	pkgJSON, netJSON, envJSON, err := marshalEnvFields(e)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		`UPDATE environments
		 SET name = ?, description = ?, packages_json = ?, networking_json = ?, env_json = ?, updated_at = ?
		 WHERE id = ?`,
		e.Name, e.Description, pkgJSON, netJSON, envJSON, time.Now().Unix(), e.ID,
	)
	return wrapErr("update environment", err)
}

func (r *environmentRepo) Delete(_ context.Context, id string) error {
	_, err := r.db.Exec(`DELETE FROM environments WHERE id = ?`, id)
	return wrapErr("delete environment", err)
}

func (r *environmentRepo) CountReferences(_ context.Context, id string) (int, error) {
	var n int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM projects WHERE environment_id = ?`, id,
	).Scan(&n)
	return n, wrapErr("count environment references", err)
}

// ── scanner helpers ────────────────────────────────────────────────────────

// envScanner is satisfied by both *sql.Row and *sql.Rows.
type envScanner interface {
	Scan(dest ...any) error
}

func scanEnv(s envScanner) (*store.Environment, error) {
	var (
		id, tenantID, ownerID, scope, name, description string
		pkgJSON, netJSON, envJSON                        string
		createdAt, updatedAt                             int64
	)
	err := s.Scan(&id, &tenantID, &ownerID, &scope, &name, &description,
		&pkgJSON, &netJSON, &envJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, wrapErr("scan environment", err)
	}

	pkg, err := encoding.UnmarshalPackageList(pkgJSON)
	if err != nil {
		return nil, err
	}
	net, err := encoding.UnmarshalNetworking(netJSON)
	if err != nil {
		return nil, err
	}
	env, err := encoding.UnmarshalStringMap(envJSON)
	if err != nil {
		return nil, err
	}

	return &store.Environment{
		ID:          id,
		TenantID:    tenantID,
		Scope:       scope,
		Name:        name,
		Description: description,
		Packages:    pkg,
		Networking:  net,
		Env:         env,
		CreatedAt:   time.Unix(createdAt, 0),
		UpdatedAt:   time.Unix(updatedAt, 0),
	}, nil
}

// ── JSON helpers ───────────────────────────────────────────────────────────

func marshalEnvFields(e *store.Environment) (pkgJSON, netJSON, envJSON string, err error) {
	pkgJSON, err = encoding.MarshalPackageList(e.Packages)
	if err != nil {
		return
	}
	netJSON, err = encoding.MarshalNetworking(e.Networking)
	if err != nil {
		return
	}
	envJSON, err = encoding.MarshalStringMap(e.Env)
	return
}
