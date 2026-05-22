package store

import (
	"context"
	"time"
)

const (
	EnvScopeTenant = "tenant"

	NetworkingUnrestricted = "unrestricted"
	NetworkingLimited      = "limited"
)

// PackageList declares packages to pre-install per package manager.
type PackageList struct {
	Pip  []string `json:"pip,omitempty"`
	Npm  []string `json:"npm,omitempty"`
	Apt  []string `json:"apt,omitempty"`
	Cargo []string `json:"cargo,omitempty"`
}

// NetworkingConfig describes the sandbox networking policy.
type NetworkingConfig struct {
	// Mode is "unrestricted" (default) or "limited".
	Mode         string   `json:"mode,omitempty"`
	AllowedHosts []string `json:"allowed_hosts,omitempty"`
}

// Environment is a named, reusable sandbox configuration template.
// All environments are tenant-scoped and managed by tenant admins.
type Environment struct {
	ID          string
	TenantID    string
	Scope       string // always EnvScopeTenant
	Name        string
	Description string
	Packages    PackageList
	Networking  NetworkingConfig
	Env         map[string]string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// EnvironmentRepository provides CRUD operations for Environment records.
type EnvironmentRepository interface {
	// Create inserts a new Environment.
	Create(ctx context.Context, e *Environment) error

	// Get returns an Environment by ID, or nil if not found.
	Get(ctx context.Context, id string) (*Environment, error)

	// List returns all tenant-scoped Environments for the given tenant.
	List(ctx context.Context, tenantID string) ([]*Environment, error)

	// Update replaces the mutable fields of an existing Environment.
	Update(ctx context.Context, e *Environment) error

	// Delete removes an Environment by ID.
	Delete(ctx context.Context, id string) error

	// CountReferences returns how many records in the given refTable reference this
	// environment_id. Used to guard against deleting referenced environments.
	CountReferences(ctx context.Context, id string) (int, error)
}
