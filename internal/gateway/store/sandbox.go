package store

import (
	"context"
	"time"
)

// SandboxRecord holds the runtime metadata for a live per-session sandbox.
// It is the source of truth shared across all worker processes so that any
// worker can locate and authenticate to an existing sandbox rather than
// creating a duplicate.
type SandboxRecord struct {
	SessionID       string // primary key (scoped internal session ID)
	TenantID        string // tenant that owns this sandbox; used for isolation and indexing
	SandboxID       string // driver-specific handle (container ID, pod name, …)
	Endpoint        string // HTTP base URL for the tool-server (e.g. "http://localhost:49321")
	Token           string // Bearer token required by the tool-server /execute endpoint
	LastSeen        int64  // unix timestamp; updated when Acquire is called
	EnvironmentSpec string // JSON-encoded resources.Environment; empty means default environment
}

// SandboxRepository persists sandbox runtime state.
type SandboxRepository interface {
	// Upsert writes or overwrites the record for r.SessionID.
	Upsert(ctx context.Context, r SandboxRecord) error

	// Get returns the record for sessionID, or (nil, nil) if not found.
	Get(ctx context.Context, sessionID string) (*SandboxRecord, error)

	// Delete removes the record for sessionID. No-op if absent.
	Delete(ctx context.Context, sessionID string) error

	// Touch updates LastSeen for sessionID to now. No-op if absent.
	Touch(ctx context.Context, sessionID string) error

	// DeleteStaleBefore removes all records whose LastSeen is before cutoff.
	// Returns the number of rows deleted.
	DeleteStaleBefore(ctx context.Context, cutoff time.Time) (int, error)

	// SetEnvironmentSpec upserts a record keyed by sessionID, updating only the
	// environment_spec column. Creates a stub record (no container) if absent.
	SetEnvironmentSpec(ctx context.Context, sessionID, spec string) error
}

// SessionResourceRecord persists a dynamic resource declaration for a session.
// Content and tokens are NOT stored here — only the metadata needed to
// re-initialize the resource when a container is rebuilt.
type SessionResourceRecord struct {
	ID         string // unique resource ID
	SessionID  string // owning session (scoped internal ID)
	Type       string // "file" | "git"
	TargetPath string // path relative to workspace where resource is materialised
	Spec       string // JSON-encoded resource spec (no secrets)
	CreatedAt  int64  // unix timestamp
}

// SessionResourceRepository persists dynamic resource declarations per session.
type SessionResourceRepository interface {
	// Upsert writes or overwrites the record for r.ID.
	Upsert(ctx context.Context, r SessionResourceRecord) error

	// List returns all resource records for sessionID.
	List(ctx context.Context, sessionID string) ([]SessionResourceRecord, error)

	// Delete removes the record for id. No-op if absent.
	Delete(ctx context.Context, id string) error

	// DeleteBySession removes all records for sessionID.
	DeleteBySession(ctx context.Context, sessionID string) error
}
