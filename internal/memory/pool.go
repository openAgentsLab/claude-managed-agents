package memory

import (
	"fmt"
	"sync"

	"forge/internal/config"
)

// namedStore wraps a MemoryStore and owns the display name and description so
// they can be updated (e.g. after a custom store rename) without recreating the
// underlying backend connection. It also patches SearchResult.StoreName so
// search results always reflect the current name.
type namedStore struct {
	mu   sync.RWMutex
	name string
	desc string
	MemoryStore // delegates List/Read/Write/Edit/Delete to the inner store
}

func (n *namedStore) Name() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.name
}

func (n *namedStore) Description() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.desc
}

// Search delegates to the inner store and overwrites StoreName so results
// always carry the current name even after a rename.
func (n *namedStore) Search(query string) ([]SearchResult, error) {
	results, err := n.MemoryStore.Search(query)
	if err != nil {
		return nil, err
	}
	n.mu.RLock()
	current := n.name
	n.mu.RUnlock()
	for i := range results {
		results[i].StoreName = current
	}
	return results, nil
}

func (n *namedStore) setMeta(name, description string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.name = name
	n.desc = description
}

// Pool manages MemoryStore instances, caching them by scope key.
type Pool struct {
	mu        sync.Mutex
	instances map[string]*namedStore
	backend   StoreBackend
}

// NewPool creates a Pool using the configured backend driver.
// Returns nil, nil when cfg.Disabled is true.
func NewPool(cfg config.MemoryConfig) (*Pool, error) {
	if cfg.Disabled {
		return nil, nil
	}
	b, err := OpenBackend(cfg)
	if err != nil {
		return nil, fmt.Errorf("memory pool: %w", err)
	}
	return &Pool{
		instances: make(map[string]*namedStore),
		backend:   b,
	}, nil
}

// UserStore returns the MemoryStore scoped to the given userID.
func (p *Pool) UserStore(userID string) MemoryStore {
	return p.get("user:"+userID, UserStoreName, UserStoreDesc)
}

// ProjectStore returns the MemoryStore scoped to the given projectID.
func (p *Pool) ProjectStore(projectID string) MemoryStore {
	return p.get("project:"+projectID, ProjectStoreName, ProjectStoreDesc)
}

// TenantStore returns the MemoryStore scoped to the given tenantID.
func (p *Pool) TenantStore(tenantID string) MemoryStore {
	return p.get("tenant:"+tenantID, TenantStoreName, TenantStoreDesc)
}

// GetCustomStore returns the MemoryStore for a custom store by its metadata.
// Name and description are always refreshed to the latest values so that a
// rename takes effect in the next session without a process restart.
func (p *Pool) GetCustomStore(meta StoreMeta) MemoryStore {
	return p.get("custom:"+meta.ID, meta.Name, meta.Description)
}

// Backend returns the underlying StoreBackend (used by StoreManager).
func (p *Pool) Backend() StoreBackend {
	return p.backend
}

func (p *Pool) get(key, name, description string) MemoryStore {
	p.mu.Lock()
	defer p.mu.Unlock()
	if s, ok := p.instances[key]; ok {
		// Always refresh name/description in case the store was renamed.
		s.setMeta(name, description)
		return s
	}
	inner := p.backend.NewStore(key, name, description)
	s := &namedStore{name: name, desc: description, MemoryStore: inner}
	p.instances[key] = s
	return s
}

// Close releases the backend connection.
func (p *Pool) Close() error {
	return p.backend.Close()
}
