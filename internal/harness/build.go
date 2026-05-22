package harness

import (
	"context"
	"fmt"

	"forge/internal/config"
	"forge/internal/gateway/session"
	"forge/internal/history"
	"forge/internal/tools"
)

// Build assembles a Harness backed by the given session store. No brain is
// attached at this level; every request injects a per-tenant brain (and history
// manager) via WithBrain / WithHistory before calling Run.
// Memory is not attached; call h.WithMemoryStores() to attach per-session stores.
func Build(_ context.Context, cfg *config.Config, _ tools.ToolRegistry) (*Harness, error) {
	store, err := session.Open(cfg.Session)
	if err != nil {
		return nil, fmt.Errorf("open session store: %w", err)
	}

	// No compaction at the global level; each session injects its own history
	// manager (built with the per-tenant model) via WithHistory.
	mgr := history.NewManager(store, nil)

	return New(nil, store, mgr), nil
}
