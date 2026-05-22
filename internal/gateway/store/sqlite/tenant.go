package sqlite

import (
	"context"
	"database/sql"
	"time"

	"forge/internal/gateway/store"
	"forge/internal/gateway/store/encoding"
)

type tenantRepo struct{ db *sql.DB }

func (r *tenantRepo) Seed(_ context.Context, t *store.Tenant) error {
	settingsJSON, err := encoding.MarshalSettings(t.Settings)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = r.db.Exec(
		`INSERT OR IGNORE INTO tenants (id, name, settings_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		t.ID, t.Name, settingsJSON, now, now,
	)
	return wrapErr("seed tenant", err)
}

func (r *tenantRepo) Get(_ context.Context, id string) (*store.Tenant, error) {
	var name, sJSON string
	var createdAt, updatedAt int64
	err := r.db.QueryRow(
		`SELECT name, settings_json, created_at, updated_at FROM tenants WHERE id = ?`, id,
	).Scan(&name, &sJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, wrapErr("get tenant", err)
	}
	settings, err := encoding.UnmarshalSettings(sJSON)
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
		var id, name, sJSON string
		var createdAt, updatedAt int64
		if err := rows.Scan(&id, &name, &sJSON, &createdAt, &updatedAt); err != nil {
			return nil, wrapErr("scan tenant row", err)
		}
		settings, err := encoding.UnmarshalSettings(sJSON)
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
		`UPDATE tenants SET settings_json = ?, updated_at = ? WHERE id = ?`,
		sJSON, time.Now().Unix(), tenantID,
	)
	return wrapErr("update tenant settings", err)
}

