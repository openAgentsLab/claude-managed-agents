package session

import (
	"fmt"

	"forge/internal/config"
)

// Factory is the constructor function for a SessionStore implementation.
// opts come from config.SessionConfig.Options, allowing drivers to read
// their own configuration keys.
type Factory func(opts map[string]string) (SessionStore, error)

var drivers = map[string]Factory{}

// Register registers a driver. Sub-packages call this from their init()
// and are activated via "import _" in main.go.
func Register(name string, f Factory) {
	if _, dup := drivers[name]; dup {
		panic("session: driver already registered: " + name)
	}
	drivers[name] = f
}

// Open creates a SessionStore from cfg. Called once at startup by main.go.
func Open(cfg config.SessionConfig) (SessionStore, error) {
	f, ok := drivers[cfg.DriverOrDefault()]
	if !ok {
		return nil, fmt.Errorf("session: unknown driver %q (forgot import?)", cfg.DriverOrDefault())
	}
	return f(cfg.Options)
}

// Drivers are registered by their sub-packages via init():
//   - "memory" → import _ "forge/internal/gateway/session/memory"
//   - "sqlite" → import _ "forge/internal/gateway/session/sqlite"
//   - "remote" → import _ "forge/internal/gateway/session/remote"  (future)
