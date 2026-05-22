package postgres

import (
	"context"
	"database/sql"
	"time"

	"forge/internal/gateway/store"
	"forge/internal/gateway/store/encoding"
)

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
		 VALUES ($1, $2, '', 'tenant', $3, $4, $5, $6, $7, $8, $9)`,
		e.ID, e.TenantID, e.Name, e.Description,
		pkgJSON, netJSON, envJSON, now, now,
	)
	return wrapErr("create environment", err)
}

func (r *environmentRepo) Get(_ context.Context, id string) (*store.Environment, error) {
	row := r.db.QueryRow(
		`SELECT id, tenant_id, owner_id, scope, name, description, packages_json, networking_json, env_json, created_at, updated_at
		 FROM environments WHERE id = $1`, id,
	)
	return scanEnv(row)
}

func (r *environmentRepo) List(_ context.Context, tenantID string) ([]*store.Environment, error) {
	rows, err := r.db.Query(
		`SELECT id, tenant_id, owner_id, scope, name, description, packages_json, networking_json, env_json, created_at, updated_at
		 FROM environments
		 WHERE tenant_id = $1 AND scope = 'tenant'
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
		 SET name = $1, description = $2, packages_json = $3, networking_json = $4, env_json = $5, updated_at = $6
		 WHERE id = $7`,
		e.Name, e.Description, pkgJSON, netJSON, envJSON, time.Now().Unix(), e.ID,
	)
	return wrapErr("update environment", err)
}

func (r *environmentRepo) Delete(_ context.Context, id string) error {
	_, err := r.db.Exec(`DELETE FROM environments WHERE id = $1`, id)
	return wrapErr("delete environment", err)
}

func (r *environmentRepo) CountReferences(_ context.Context, id string) (int, error) {
	var n int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM projects WHERE environment_id = $1`, id,
	).Scan(&n)
	return n, wrapErr("count environment references", err)
}

// ── scanner ────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEnv(s rowScanner) (*store.Environment, error) {
	var (
		id, tenantID, ownerID, scope, name, description string
		pkgJSON, netJSON, envJSON                        []byte
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

	pkg, err := encoding.UnmarshalPackageList(string(pkgJSON))
	if err != nil {
		return nil, err
	}
	net, err := encoding.UnmarshalNetworking(string(netJSON))
	if err != nil {
		return nil, err
	}
	env, err := encoding.UnmarshalStringMap(string(envJSON))
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
