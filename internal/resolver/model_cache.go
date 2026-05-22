package resolver

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"

	"forge/internal/config"
	"forge/internal/brain"
)

// modelCache lazily creates and caches model.ToolCallingChatModel instances
// keyed by ModelCacheKey. Identical configs share one HTTP client and
// connection pool rather than creating a new one per session.
//
// All methods are safe for concurrent use.
type modelCache struct {
	mu     sync.RWMutex
	models map[string]model.ToolCallingChatModel
}

func newModelCache() *modelCache {
	return &modelCache{models: make(map[string]model.ToolCallingChatModel)}
}

// getOrCreate returns a cached model for cfg, creating it on first use.
func (c *modelCache) getOrCreate(ctx context.Context, cfg config.ModelConfig) (model.ToolCallingChatModel, error) {
	key := ModelCacheKey(cfg)

	c.mu.RLock()
	m, ok := c.models[key]
	c.mu.RUnlock()
	if ok {
		return m, nil
	}

	// Double-checked locking: re-verify under write lock before allocating.
	c.mu.Lock()
	defer c.mu.Unlock()
	if m, ok = c.models[key]; ok {
		return m, nil
	}
	m, err := brain.NewToolCallingModel(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("resolver: create model %s/%s: %w", cfg.Provider, cfg.Model, err)
	}
	c.models[key] = m
	return m, nil
}
