package store

import (
	"context"
	"time"
)

// SecretMeta is the metadata returned by SecretRepository.List.
// The plaintext value is never exposed through this interface.
type SecretMeta struct {
	Name        string
	Description string
	UpdatedAt   time.Time
}

// SecretRepository provides per-user and per-tenant encrypted secret storage.
// Values are AES-256-GCM encrypted at rest; plaintext is only accessible
// via Resolve. All write operations require a non-nil master key.
//
// Tenant-level secrets use userID="". Resolve checks user-level first and
// falls back to tenant-level when a user-scoped secret is not found.
type SecretRepository interface {
	// Set creates or overwrites the named secret.
	// Pass userID="" to store a tenant-level secret (admin only).
	Set(ctx context.Context, tenantID, userID, name, description, plaintext string) error

	// List returns metadata for all secrets belonging to the given scope.
	// Pass userID="" to list tenant-level secrets.
	// Returns an empty (non-nil) slice when none exist.
	List(ctx context.Context, tenantID, userID string) ([]SecretMeta, error)

	// Delete removes the secret. Returns nil if the secret does not exist.
	Delete(ctx context.Context, tenantID, userID, name string) error

	// Resolve returns the plaintext for ref. If ref starts with "vault:", the
	// suffix is treated as a secret name: user-level is checked first, then
	// tenant-level. Any other string is returned as-is (literal pass-through).
	Resolve(ctx context.Context, tenantID, userID, ref string) (string, error)
}
