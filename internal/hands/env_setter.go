package hands

import (
	"context"
	"encoding/json"
	"fmt"

	"forge/internal/gateway/store"
	"forge/internal/resources"
)

// SessionEnvSetter is an optional interface implemented by Pool drivers that
// support pre-configuring the sandbox environment for a session before its
// container starts. Callers should type-assert from Pool:
//
//	if es, ok := pool.(hands.SessionEnvSetter); ok { ... }
type SessionEnvSetter interface {
	// SetSessionEnvironment stores the resolved Environment for sessionID.
	// It is applied the next time a container is started for this session.
	SetSessionEnvironment(ctx context.Context, sessionID string, env resources.Environment) error
}

// SetEnvSpec serialises env to JSON and writes it to the sandbox record.
// Shared by all Pool drivers; no-op when repo is nil.
func SetEnvSpec(ctx context.Context, repo store.SandboxRepository, sessionID string, env resources.Environment) error {
	if repo == nil {
		return nil
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal environment: %w", err)
	}
	return repo.SetEnvironmentSpec(ctx, sessionID, string(raw))
}

// LoadEnvSpec reads the persisted environment spec for sessionID.
// Returns the deserialised Environment and the raw JSON string (needed by
// drivers that re-use the spec when upserting a new sandbox record).
// Both return values are zero/empty when repo is nil or no record exists.
func LoadEnvSpec(ctx context.Context, repo store.SandboxRepository, sessionID string) (resources.Environment, string) {
	if repo == nil {
		return resources.Environment{}, ""
	}
	rec, err := repo.Get(ctx, sessionID)
	if err != nil || rec == nil || rec.EnvironmentSpec == "" {
		return resources.Environment{}, ""
	}
	var env resources.Environment
	_ = json.Unmarshal([]byte(rec.EnvironmentSpec), &env)
	return env, rec.EnvironmentSpec
}
