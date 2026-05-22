package postgres

import (
	"context"
	"database/sql"
	"time"

	"forge/internal/gateway/store"
	"forge/internal/gateway/store/encoding"
)

type tenantRepo struct{ db *sql.DB }

func (r *tenantRepo) Seed(_ context.Context, t *store.Tenant) error {
	sJSON, err := encoding.MarshalSettings(t.Settings)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = r.db.Exec(
		`INSERT INTO tenants (id, name, settings_json, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO NOTHING`,
		t.ID, t.Name, sJSON, now, now,
	)
	return wrapErr("seed tenant", err)
}

func (r *tenantRepo) Get(_ context.Context, id string) (*store.Tenant, error) {
	var name string
	var sJSON []byte
	var createdAt, updatedAt int64
	err := r.db.QueryRow(
		`SELECT name, settings_json, created_at, updated_at FROM tenants WHERE id = $1`, id,
	).Scan(&name, &sJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, wrapErr("get tenant", err)
	}
	settings, err := encoding.UnmarshalSettings(string(sJSON))
	if err != nil {
		return nil, err
	}
	return &store.Tenant{
		ID:        id,
		Name:      name,
		Settings:  settings,
		CreatedAt: time.Unix(createdAt, 0),
		UpdatedAt: time.Unix(updatedAt, 0),
	}, nil
}

func (r *tenantRepo) List(_ context.Context) ([]*store.Tenant, error) {
	rows, err := r.db.Query(
		`SELECT id, name, settings_json, created_at, updated_at FROM tenants ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, wrapErr("list tenants", err)
	}
	defer rows.Close()

	var out []*store.Tenant
	for rows.Next() {
		var id, name string
		var sJSON []byte
		var createdAt, updatedAt int64
		if err := rows.Scan(&id, &name, &sJSON, &createdAt, &updatedAt); err != nil {
			return nil, wrapErr("scan tenant row", err)
		}
		settings, err := encoding.UnmarshalSettings(string(sJSON))
		if err != nil {
			return nil, err
		}
		out = append(out, &store.Tenant{
			ID:        id,
			Name:      name,
			Settings:  settings,
			CreatedAt: time.Unix(createdAt, 0),
			UpdatedAt: time.Unix(updatedAt, 0),
		})
	}
	return out, rows.Err()
}

func (r *tenantRepo) UpdateSettings(_ context.Context, tenantID string, s store.Settings) error {
	sJSON, err := encoding.MarshalSettings(s)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		`UPDATE tenants SET settings_json = $1, updated_at = $2 WHERE id = $3`,
		sJSON, time.Now().Unix(), tenantID,
	)
	return wrapErr("update tenant settings", err)
}
