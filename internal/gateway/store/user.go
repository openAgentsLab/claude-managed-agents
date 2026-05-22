package store

import (
	"context"
	"time"
)

// UserSettings holds optional per-user LLM and inference overrides.
// Users can personalise their model endpoint and brain behaviour within the
// bounds allowed by their tenant. Persisted as a JSON blob in the DB.
type UserSettings struct {
	// ModelOverride optionally routes this user to a different LLM endpoint.
	// Non-empty fields replace the corresponding tenant/global value.
	ModelOverride *ModelSettings `json:"model,omitempty"`
	// BrainOverride optionally tunes inference parameters for this user.
	BrainOverride *BrainSettings `json:"brain,omitempty"`
}

// User is a user credential and role record within a tenant.
type User struct {
	TenantID     string
	Username     string
	PasswordHash string
	Role         string // "admin" | "member" | "viewer"
	Settings     UserSettings
	CreatedAt    time.Time
}

// UserRepository provides CRUD operations for tenant users.
type UserRepository interface {
	// Seed inserts a user if it does not already exist (INSERT OR IGNORE).
	Seed(ctx context.Context, u *User) error

	// Get returns a user within a tenant, or nil if not found.
	Get(ctx context.Context, tenantID, username string) (*User, error)

	// FindByUsername searches all tenants for a username.
	// Returns nil, nil, nil when not found.
	FindByUsername(ctx context.Context, username string) (*Tenant, *User, error)

	// List returns all users within a tenant ordered by creation time.
	List(ctx context.Context, tenantID string) ([]*User, error)

	// UpdateRole persists a role change for a user.
	UpdateRole(ctx context.Context, tenantID, username, role string) error

	// GetSettings returns the current UserSettings for a user.
	GetSettings(ctx context.Context, tenantID, username string) (UserSettings, error)

	// UpdateSettings persists a user-level settings change.
	UpdateSettings(ctx context.Context, tenantID, username string, s UserSettings) error
}
