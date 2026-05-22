package memory

import (
	"errors"
	"fmt"
	"regexp"

	"forge/internal/config"
)

// ValidFilename matches acceptable memory document filenames.
var ValidFilename = regexp.MustCompile(`^[a-z0-9_\-]+\.md$`)

// ValidateFilename returns an error when filename does not match ValidFilename.
func ValidateFilename(filename string) error {
	if !ValidFilename.MatchString(filename) {
		return fmt.Errorf("memory: invalid filename %q (must match ^[a-z0-9_\\-]+\\.md$)", filename)
	}
	return nil
}

// Document is a single memory entry returned by Read.
type Document struct {
	Filename string
	Content  string
	SHA256   string
	Version  int
}

// DocumentSummary is a compact representation used in list indexes.
type DocumentSummary struct {
	Filename string
	Excerpt  string // first non-empty line of content
}

// SearchResult is one match returned by Search.
type SearchResult struct {
	StoreName string
	Filename  string
	Excerpt   string
}

// StoreMeta holds metadata for a custom (user-created) memory store.
type StoreMeta struct {
	ID          string
	Name        string
	Description string
	Visibility  string // "private" | "shared_tenant"
	WritePolicy string // "owner_only" | "members"
	CreatedBy   string // scoped userID e.g. "tenant1/alice"
	CreatedAt   int64  // unix seconds
}

// StoreInfo is the public representation returned to callers.
type StoreInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
	WritePolicy string `json:"write_policy"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   int64  `json:"created_at"`
}

// ConflictError is returned when a sha256 precondition fails during write/edit.
type ConflictError struct {
	CurrentSHA256 string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("memory: sha256 conflict, current sha256: %s", e.CurrentSHA256)
}

// MaxDocumentBytes is the maximum allowed size for a single memory document.
const MaxDocumentBytes = 100 * 1024 // 100 KB

var (
	ErrNotFound  = errors.New("memory: document not found")
	ErrForbidden = errors.New("memory: write not permitted on this store")
)

// ValidateContent returns an error when content exceeds MaxDocumentBytes.
func ValidateContent(content string) error {
	if len(content) > MaxDocumentBytes {
		return fmt.Errorf("memory: document exceeds maximum size of %d bytes (%d bytes given)", MaxDocumentBytes, len(content))
	}
	return nil
}

// validateVisibility returns an error for unrecognised visibility values.
// "shared_project" is reserved but not yet implemented.
func validateVisibility(v string) error {
	switch v {
	case "private", "shared_tenant":
		return nil
	case "shared_project":
		return fmt.Errorf("memory: visibility %q is not yet supported", v)
	default:
		return fmt.Errorf("memory: invalid visibility %q (must be 'private' or 'shared_tenant')", v)
	}
}

// validateWritePolicy returns an error for unrecognised write_policy values.
func validateWritePolicy(wp string) error {
	switch wp {
	case "owner_only", "members":
		return nil
	default:
		return fmt.Errorf("memory: invalid write_policy %q (must be 'owner_only' or 'members')", wp)
	}
}

// MemoryStore is the document-level interface implemented by all store backends.
// Each instance is scoped to a single store (user/project/tenant/custom) and
// operates on documents within that scope only.
type MemoryStore interface {
	// Name returns the store's display identifier used in tool addressing.
	Name() string

	// Description returns a human-readable explanation of this store's purpose.
	// Injected into the system prompt and memory_list output so the LLM can
	// choose the correct store when writing.
	Description() string

	// List returns summaries of all documents in the store.
	List() ([]DocumentSummary, error)

	// Read returns the full document. found is false (no error) when the filename
	// does not exist.
	Read(filename string) (Document, bool, error)

	// Search runs full-text search and returns at most 5 results.
	Search(query string) ([]SearchResult, error)

	// Write creates or fully replaces a document (upsert-replace, not append).
	// sha256 is the expected current content hash for optimistic concurrency;
	// pass "" to force-overwrite without checking. Returns the new sha256.
	Write(filename, content, sha256 string) (newSHA string, err error)

	// Edit applies a str_replace to the document's content. oldStr must appear
	// exactly once; 0 or 2+ matches returns an error. sha256 works like Write.
	// Returns the new sha256.
	Edit(filename, oldStr, newStr, sha256 string) (newSHA string, err error)

	// Delete removes a document. Silently succeeds when filename does not exist.
	Delete(filename string) error
}

// StoreBackend is the global DB connection owned by Pool.
// It creates MemoryStore instances and manages custom store metadata.
type StoreBackend interface {
	// NewStore returns a MemoryStore scoped to the given storeID.
	// Multiple calls with the same storeID may return distinct instances
	// sharing the same underlying connection.
	NewStore(storeID, name, description string) MemoryStore

	// CreateMeta persists custom store metadata.
	CreateMeta(meta StoreMeta) error

	// GetMeta returns metadata for the given store ID.
	// found is false (no error) when the ID does not exist.
	GetMeta(id string) (StoreMeta, bool, error)

	// ListMeta returns stores accessible to the caller.
	// Includes stores where created_by == callerUserID (private) or
	// visibility == "shared_tenant" and tenant prefix matches tenantID.
	ListMeta(callerUserID, tenantID string) ([]StoreMeta, error)

	// UpdateMeta overwrites mutable fields of a custom store.
	UpdateMeta(id, name, description, visibility, writePolicy string) error

	// DeleteMeta removes the store metadata row and all its documents.
	DeleteMeta(id string) error

	Close() error
}

// BackendFactory creates a StoreBackend from driver options.
type BackendFactory func(opts map[string]string) (StoreBackend, error)

var (
	registeredBackends = map[string]BackendFactory{}
)

// Register registers a StoreBackend driver. Called from driver init() functions.
func Register(name string, f BackendFactory) {
	if _, ok := registeredBackends[name]; ok {
		panic("memory: backend already registered: " + name)
	}
	registeredBackends[name] = f
}

// OpenBackend opens a StoreBackend using the driver named in cfg.
// Returns nil, nil when cfg.Disabled is true.
func OpenBackend(cfg config.MemoryConfig) (StoreBackend, error) {
	if cfg.Disabled {
		return nil, nil
	}
	driver := cfg.Driver
	if driver == "" {
		driver = "sqlite"
	}
	f, ok := registeredBackends[driver]
	if !ok {
		return nil, fmt.Errorf("memory: unknown backend driver %q", driver)
	}
	return f(cfg.Options)
}
