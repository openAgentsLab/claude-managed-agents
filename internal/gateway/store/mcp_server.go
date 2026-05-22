package store

import (
	"context"
	"time"
)

// MCPServerRecord is a tenant-scoped MCP server configuration stored in the DB.
// Env and Headers values may be vault references ("vault:name") resolved at
// runtime; plaintext values are also accepted.
type MCPServerRecord struct {
	TenantID  string
	Name      string
	Type      string            // stdio | sse | http | ws
	Command   string            // stdio only
	Args      []string          // stdio only
	Env       map[string]string // may contain "vault:xxx" references
	URL       string            // remote only
	Headers   map[string]string // may contain "vault:xxx" references
	Disabled  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MCPServerRepository provides CRUD operations for tenant MCP server configs.
type MCPServerRepository interface {
	// Upsert inserts or updates (by tenant+name) an MCP server config.
	Upsert(ctx context.Context, r *MCPServerRecord) error

	// Get returns the record, or nil if not found.
	Get(ctx context.Context, tenantID, name string) (*MCPServerRecord, error)

	// List returns all MCP server configs for the tenant, ordered by name.
	List(ctx context.Context, tenantID string) ([]*MCPServerRecord, error)

	// Delete removes the record. Returns nil if not found.
	Delete(ctx context.Context, tenantID, name string) error
}
