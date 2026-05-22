package store

import (
	"context"
	"time"
)

// ModelSettings holds optional per-tenant LLM endpoint overrides.
// Non-empty fields replace the corresponding global ModelConfig value.
type ModelSettings struct {
	Provider   string `json:"provider,omitempty"`
	APIKey     string `json:"api_key,omitempty"`
	BaseURL    string `json:"base_url,omitempty"`
	Model      string `json:"model,omitempty"`
	ByAzure    bool   `json:"by_azure,omitempty"`
	APIVersion string `json:"api_version,omitempty"`
}

// BrainSettings holds optional inference parameter overrides.
// Model selection belongs in ModelSettings, not here.
type BrainSettings struct {
	Effort     string `json:"effort,omitempty"`
	Thinking   string `json:"thinking,omitempty"` // "adaptive" | "disabled"
	MaxRetries int    `json:"max_retries,omitempty"`
}

// Settings holds the permission policy, resource limits, and optional model /
// brain overrides for a tenant. Persisted as a JSON blob in the DB.
type Settings struct {
	AllowRules []string `json:"allow_rules,omitempty"`
	DenyRules      []string `json:"deny_rules,omitempty"`
	MemoryBytes    int64    `json:"memory_bytes,omitempty"` // 0 = unlimited
	NanoCPUs       int64    `json:"nano_cpus,omitempty"`    // 0 = unlimited; 1 CPU = 1_000_000_000
	// ModelOverride optionally routes this tenant to a different LLM endpoint.
	ModelOverride *ModelSettings `json:"model,omitempty"`
	// BrainOverride optionally tunes inference parameters for this tenant.
	BrainOverride *BrainSettings `json:"brain,omitempty"`
}

// Tenant is a persistent tenant record.
type Tenant struct {
	ID        string
	Name      string
	Settings  Settings
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TenantRepository provides CRUD operations for tenants.
type TenantRepository interface {
	// Seed inserts a tenant if it does not already exist (INSERT OR IGNORE).
	Seed(ctx context.Context, t *Tenant) error

	// Get returns the tenant by ID, or nil if not found.
	Get(ctx context.Context, id string) (*Tenant, error)

	// List returns all tenants ordered by creation time.
	List(ctx context.Context) ([]*Tenant, error)

	// UpdateSettings persists a runtime settings change.
	UpdateSettings(ctx context.Context, tenantID string, s Settings) error
}
