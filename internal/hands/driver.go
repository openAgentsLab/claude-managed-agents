package hands

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"

	"forge/internal/config"
	"forge/internal/gateway/store"
)

// PoolDeps bundles optional dependencies a Pool driver may use.
// All fields are optional; nil means the driver operates without that capability.
type PoolDeps struct {
	Sandbox   store.SandboxRepository        // sandbox record persistence (container ID, token, endpoint)
	Resources store.SessionResourceRepository // dynamic resource declaration persistence
	Sandboxed []tool.InvokableTool            // in-process tool set; local driver provisions these into its shared sandbox; remote drivers ignore it
}

// PoolFactory creates a Pool for a driver.
type PoolFactory func(ctx context.Context, cfg config.SandboxConfig, deps PoolDeps) (Pool, error)

var poolFactories = map[string]PoolFactory{}

// RegisterPool registers a pool factory for a driver. Call from init() in
// each driver package to activate per-session sandbox pooling for that driver.
func RegisterPool(name string, f PoolFactory) {
	if _, dup := poolFactories[name]; dup {
		panic("hands: pool factory already registered: " + name)
	}
	poolFactories[name] = f
}

// OpenPool looks up and calls the pool factory for the driver.
// Returns an error if no factory is registered (unknown or unimported driver).
func OpenPool(ctx context.Context, driver string, cfg config.SandboxConfig, deps PoolDeps) (Pool, error) {
	f, ok := poolFactories[driver]
	if !ok {
		return nil, fmt.Errorf("hands: no pool factory for driver %q (forgot import?)", driver)
	}
	return f(ctx, cfg, deps)
}

// Pool drivers are registered by their sub-packages via init():
//   - "local"  → import _ "forge/internal/hands/local"
//   - "docker" → import _ "forge/internal/hands/docker"
//   - "k8s"   → import _ "forge/internal/hands/k8s"
