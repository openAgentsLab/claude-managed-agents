package memory

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateStoreRequest carries fields for creating a custom store.
type CreateStoreRequest struct {
	Name        string
	Description string
	Visibility  string // "private" | "shared_tenant"
	WritePolicy string // "owner_only" | "members"
	CreatedBy   string
}

// UpdateStoreRequest carries mutable fields for a custom store update.
type UpdateStoreRequest struct {
	Name        string
	Description string
	Visibility  string
	WritePolicy string
}

// StoreManager handles CRUD for custom memory stores.
type StoreManager struct {
	pool *Pool
}

// NewStoreManager creates a StoreManager backed by the given Pool.
func NewStoreManager(pool *Pool) *StoreManager {
	return &StoreManager{pool: pool}
}

// Create persists a new custom store and returns its generated ID.
func (m *StoreManager) Create(req CreateStoreRequest) (string, error) {
	if req.Name == "" {
		return "", fmt.Errorf("memory: store name is required")
	}
	if req.Description == "" {
		return "", fmt.Errorf("memory: store description is required")
	}
	visibility := req.Visibility
	if visibility == "" {
		visibility = "private"
	}
	if err := validateVisibility(visibility); err != nil {
		return "", err
	}
	writePolicy := req.WritePolicy
	if writePolicy == "" {
		writePolicy = "owner_only"
	}
	if err := validateWritePolicy(writePolicy); err != nil {
		return "", err
	}

	meta := StoreMeta{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Visibility:  visibility,
		WritePolicy: writePolicy,
		CreatedBy:   req.CreatedBy,
		CreatedAt:   time.Now().Unix(),
	}
	if err := m.pool.Backend().CreateMeta(meta); err != nil {
		return "", fmt.Errorf("memory: create store: %w", err)
	}
	return meta.ID, nil
}

// List returns all stores accessible to the caller.
func (m *StoreManager) List(callerUserID, tenantID string) ([]StoreInfo, error) {
	metas, err := m.pool.Backend().ListMeta(callerUserID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("memory: list stores: %w", err)
	}
	infos := make([]StoreInfo, len(metas))
	for i, meta := range metas {
		infos[i] = metaToInfo(meta)
	}
	return infos, nil
}

// Get returns metadata for a single store by ID.
// Returns ErrNotFound when the store does not exist.
func (m *StoreManager) Get(id string) (StoreInfo, error) {
	meta, found, err := m.pool.Backend().GetMeta(id)
	if err != nil {
		return StoreInfo{}, fmt.Errorf("memory: get store: %w", err)
	}
	if !found {
		return StoreInfo{}, ErrNotFound
	}
	return metaToInfo(meta), nil
}

// Update overwrites mutable fields of a custom store.
// Only the creator may update; the caller must verify ownership before calling.
func (m *StoreManager) Update(id string, req UpdateStoreRequest) error {
	if req.Visibility != "" {
		if err := validateVisibility(req.Visibility); err != nil {
			return err
		}
	}
	if req.WritePolicy != "" {
		if err := validateWritePolicy(req.WritePolicy); err != nil {
			return err
		}
	}
	if err := m.pool.Backend().UpdateMeta(id, req.Name, req.Description, req.Visibility, req.WritePolicy); err != nil {
		return fmt.Errorf("memory: update store: %w", err)
	}
	return nil
}

// Delete removes the store and all its documents.
// Only the creator may delete; the caller must verify ownership before calling.
func (m *StoreManager) Delete(id string) error {
	if err := m.pool.Backend().DeleteMeta(id); err != nil {
		return fmt.Errorf("memory: delete store: %w", err)
	}
	return nil
}

// ResolveForSession returns a MemoryStore instance for a custom store,
// plus whether the caller may write to it.
func (m *StoreManager) ResolveForSession(storeID, callerUserID string) (MemoryStore, bool, error) {
	meta, found, err := m.pool.Backend().GetMeta(storeID)
	if err != nil {
		return nil, false, fmt.Errorf("memory: resolve store: %w", err)
	}
	if !found {
		return nil, false, ErrNotFound
	}
	writable := meta.WritePolicy == "members" ||
		(meta.WritePolicy == "owner_only" && meta.CreatedBy == callerUserID)
	return m.pool.GetCustomStore(meta), writable, nil
}

func metaToInfo(m StoreMeta) StoreInfo {
	return StoreInfo{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Visibility:  m.Visibility,
		WritePolicy: m.WritePolicy,
		CreatedBy:   m.CreatedBy,
		CreatedAt:   m.CreatedAt,
	}
}
