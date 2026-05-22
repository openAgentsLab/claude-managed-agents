package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"forge/internal/gateway/store"
	"forge/internal/gateway/store/encoding"
)

type userRepo struct{ db *sql.DB }

func (r *userRepo) Seed(_ context.Context, u *store.User) error {
	sJSON, err := encoding.MarshalUserSettings(u.Settings)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = r.db.Exec(
		`INSERT INTO tenant_users (tenant_id, username, password_hash, role, settings_json, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (tenant_id, username) DO NOTHING`,
		u.TenantID, u.Username, u.PasswordHash, u.Role, sJSON, now,
	)
	return wrapErr("seed user", err)
}

func (r *userRepo) Get(_ context.Context, tenantID, username string) (*store.User, error) {
	var hash, role string
	var sJSON []byte
	var createdAt int64
	err := r.db.QueryRow(
		`SELECT password_hash, role, settings_json, created_at
		 FROM tenant_users WHERE tenant_id = $1 AND username = $2`,
		tenantID, username,
	).Scan(&hash, &role, &sJSON, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, wrapErr("get user", err)
	}
	settings, err := encoding.UnmarshalUserSettings(string(sJSON))
	if err != nil {
		return nil, err
	}
	return &store.User{
		TenantID:     tenantID,
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		Settings:     settings,
		CreatedAt:    time.Unix(createdAt, 0),
	}, nil
}

func (r *userRepo) FindByUsername(_ context.Context, username string) (*store.Tenant, *store.User, error) {
	var tenantID, tenantName string
	var tenantSJSON, userSJSON []byte
	var tenantCreatedAt, tenantUpdatedAt int64
	var hash, role string
	var userCreatedAt int64

	err := r.db.QueryRow(`
		SELECT t.id, t.name, t.settings_json, t.created_at, t.updated_at,
		       u.password_hash, u.role, u.settings_json, u.created_at
		FROM tenant_users u
		JOIN tenants t ON t.id = u.tenant_id
		WHERE u.username = $1
		LIMIT 1
	`, username).Scan(
		&tenantID, &tenantName, &tenantSJSON, &tenantCreatedAt, &tenantUpdatedAt,
		&hash, &role, &userSJSON, &userCreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, wrapErr("find user by username", err)
	}
	tenantSettings, err := encoding.UnmarshalSettings(string(tenantSJSON))
	if err != nil {
		return nil, nil, err
	}
	userSettings, err := encoding.UnmarshalUserSettings(string(userSJSON))
	if err != nil {
		return nil, nil, err
	}
	t := &store.Tenant{
		ID:        tenantID,
		Name:      tenantName,
		Settings:  tenantSettings,
		CreatedAt: time.Unix(tenantCreatedAt, 0),
		UpdatedAt: time.Unix(tenantUpdatedAt, 0),
	}
	u := &store.User{
		TenantID:     tenantID,
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		Settings:     userSettings,
		CreatedAt:    time.Unix(userCreatedAt, 0),
	}
	return t, u, nil
}

func (r *userRepo) List(_ context.Context, tenantID string) ([]*store.User, error) {
	rows, err := r.db.Query(
		`SELECT username, password_hash, role, created_at
		 FROM tenant_users WHERE tenant_id = $1 ORDER BY created_at ASC`,
		tenantID,
	)
	if err != nil {
		return nil, wrapErr("list users", err)
	}
	defer rows.Close()

	var out []*store.User
	for rows.Next() {
		var username, hash, role string
		var createdAt int64
		if err := rows.Scan(&username, &hash, &role, &createdAt); err != nil {
			return nil, wrapErr("scan user row", err)
		}
		out = append(out, &store.User{
			TenantID:     tenantID,
			Username:     username,
			PasswordHash: hash,
			Role:         role,
			CreatedAt:    time.Unix(createdAt, 0),
		})
	}
	return out, rows.Err()
}

func (r *userRepo) UpdateRole(_ context.Context, tenantID, username, role string) error {
	res, err := r.db.Exec(
		`UPDATE tenant_users SET role = $1 WHERE tenant_id = $2 AND username = $3`,
		role, tenantID, username,
	)
	if err != nil {
		return wrapErr("update user role", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("postgres store: user %q not found in tenant %q", username, tenantID)
	}
	return nil
}

func (r *userRepo) GetSettings(_ context.Context, tenantID, username string) (store.UserSettings, error) {
	var sJSON []byte
	err := r.db.QueryRow(
		`SELECT settings_json FROM tenant_users WHERE tenant_id = $1 AND username = $2`,
		tenantID, username,
	).Scan(&sJSON)
	if err == sql.ErrNoRows {
		return store.UserSettings{}, nil
	}
	if err != nil {
		return store.UserSettings{}, wrapErr("get user settings", err)
	}
	return encoding.UnmarshalUserSettings(string(sJSON))
}

func (r *userRepo) UpdateSettings(_ context.Context, tenantID, username string, s store.UserSettings) error {
	sJSON, err := encoding.MarshalUserSettings(s)
	if err != nil {
		return err
	}
	res, err := r.db.Exec(
		`UPDATE tenant_users SET settings_json = $1 WHERE tenant_id = $2 AND username = $3`,
		sJSON, tenantID, username,
	)
	if err != nil {
		return wrapErr("update user settings", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("postgres store: user %q not found in tenant %q", username, tenantID)
	}
	return nil
}
