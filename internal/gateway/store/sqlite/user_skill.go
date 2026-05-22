package sqlite

import (
	"context"
	"database/sql"
	"time"

	"forge/internal/gateway/store"
)

type userSkillRepo struct{ db *sql.DB }

func (r *userSkillRepo) Upsert(_ context.Context, rec *store.UserSkillRecord) error {
	now := time.Now().Unix()
	_, err := r.db.Exec(`
		INSERT INTO user_skills (tenant_id, user_id, name, content, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, user_id, name) DO UPDATE SET
			content    = excluded.content,
			updated_at = excluded.updated_at
	`, rec.TenantID, rec.UserID, rec.Name, rec.Content, now, now)
	return wrapErr("upsert user skill", err)
}

func (r *userSkillRepo) Get(_ context.Context, tenantID, userID, name string) (*store.UserSkillRecord, error) {
	var content string
	var createdAt, updatedAt int64
	err := r.db.QueryRow(`
		SELECT content, created_at, updated_at
		FROM user_skills WHERE tenant_id = ? AND user_id = ? AND name = ?
	`, tenantID, userID, name).Scan(&content, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, wrapErr("get user skill", err)
	}
	return &store.UserSkillRecord{
		TenantID:  tenantID,
		UserID:    userID,
		Name:      name,
		Content:   content,
		CreatedAt: time.Unix(createdAt, 0),
		UpdatedAt: time.Unix(updatedAt, 0),
	}, nil
}

func (r *userSkillRepo) List(_ context.Context, tenantID, userID string) ([]*store.UserSkillRecord, error) {
	rows, err := r.db.Query(`
		SELECT name, content, created_at, updated_at
		FROM user_skills WHERE tenant_id = ? AND user_id = ? ORDER BY name ASC
	`, tenantID, userID)
	if err != nil {
		return nil, wrapErr("list user skills", err)
	}
	defer rows.Close()

	var out []*store.UserSkillRecord
	for rows.Next() {
		var name, content string
		var createdAt, updatedAt int64
		if err := rows.Scan(&name, &content, &createdAt, &updatedAt); err != nil {
			return nil, wrapErr("scan user skill row", err)
		}
		out = append(out, &store.UserSkillRecord{
			TenantID:  tenantID,
			UserID:    userID,
			Name:      name,
			Content:   content,
			CreatedAt: time.Unix(createdAt, 0),
			UpdatedAt: time.Unix(updatedAt, 0),
		})
	}
	return out, rows.Err()
}

func (r *userSkillRepo) Delete(_ context.Context, tenantID, userID, name string) error {
	_, err := r.db.Exec(
		`DELETE FROM user_skills WHERE tenant_id = ? AND user_id = ? AND name = ?`,
		tenantID, userID, name,
	)
	return wrapErr("delete user skill", err)
}
