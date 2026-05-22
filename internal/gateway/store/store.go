package store

import (
	"fmt"
	"sync"
)

// Store is the top-level app data store.
// It owns a single database connection and exposes per-entity repositories.
// Adding a new entity means adding a new Repository method here and
// implementing it in the driver — no structural changes required.
type Store interface {
	Tenants()                        TenantRepository
	Users()                          UserRepository
	MCPServers()                     MCPServerRepository
	UserSkills()                     UserSkillRepository
	Agents()                         AgentRepository
	Secrets(masterKey []byte)        SecretRepository
	Sandboxes()                      SandboxRepository
	SessionResources()               SessionResourceRepository
	Environments()                   EnvironmentRepository
	Projects(masterKey []byte)       ProjectRepository
	Close()                          error
}

// DriverFactory creates a Store from driver-specific options.
type DriverFactory func(opts map[string]string) (Store, error)

var (
	mu       sync.RWMutex
	registry = map[string]DriverFactory{}
)

// Register makes a driver available. Called from driver package init() functions.
func Register(name string, f DriverFactory) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[name]; dup {
		panic("store: driver already registered: " + name)
	}
	registry[name] = f
}

// Open creates a Store using the named driver.
func Open(driver string, opts map[string]string) (Store, error) {
	mu.RLock()
	f, ok := registry[driver]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("store: unknown driver %q (forgot import?)", driver)
	}
	return f(opts)
}
