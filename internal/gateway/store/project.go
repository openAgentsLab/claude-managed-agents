package store

import (
	"context"
	"time"
)

// GitConfig holds git repository configuration for a Project.
// Token is write-only: stored encrypted at rest, never returned in API responses.
// Username is the HTTP Basic Auth user (required for Bitbucket / self-hosted Git;
// for GitHub PAT "x-access-token" is used when Username is empty).
type GitConfig struct {
	URL      string `json:"url,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Username string `json:"username,omitempty"`
	Token    string `json:"token,omitempty"`
}

// RefFile is a reference file URL that gets fetched into the session workspace
// at container start via curl/wget. Path is the destination inside the workspace;
// defaults to the URL's filename when empty.
type RefFile struct {
	URL  string `json:"url"`
	Path string `json:"path,omitempty"`
}

// Project is a user-owned resource container that groups Sessions under a
// common git repository and environment configuration.
// Hierarchy: Tenant → User → Project → Session.
type Project struct {
	ID            string
	TenantID      string
	OwnerID       string // username
	Name          string
	Description   string
	GitConfig     GitConfig
	EnvironmentID string            // optional reference to environments.id
	RefFiles      []RefFile         // reference files fetched into workspace at session start
	Env           map[string]string // inline env-var overrides, applied on top of envId env
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ProjectRepository provides CRUD operations for Project records.
type ProjectRepository interface {
	// Create inserts a new Project.
	Create(ctx context.Context, p *Project) error

	// Get returns a Project by ID, or nil if not found.
	Get(ctx context.Context, id string) (*Project, error)

	// List returns all Projects belonging to the given user within a tenant.
	List(ctx context.Context, tenantID, ownerID string) ([]*Project, error)

	// Update replaces the mutable fields of an existing Project.
	Update(ctx context.Context, p *Project) error

	// Delete removes a Project by ID.
	Delete(ctx context.Context, id string) error
}
