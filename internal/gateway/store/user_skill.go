package store

import (
	"context"
	"time"
)

// UserSkillRecord is a user-owned Skill stored in the DB.
// Content is the full SKILL.md text (frontmatter + body).
type UserSkillRecord struct {
	TenantID  string
	UserID    string
	Name      string
	Content   string // full SKILL.md content
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserSkillRepository provides CRUD operations for user skills.
type UserSkillRepository interface {
	// Upsert inserts or updates (by tenant+user+name) a skill.
	Upsert(ctx context.Context, r *UserSkillRecord) error

	// Get returns the record, or nil if not found.
	Get(ctx context.Context, tenantID, userID, name string) (*UserSkillRecord, error)

	// List returns all skills for the user, ordered by name.
	List(ctx context.Context, tenantID, userID string) ([]*UserSkillRecord, error)

	// Delete removes the record. Returns nil if not found.
	Delete(ctx context.Context, tenantID, userID, name string) error
}
