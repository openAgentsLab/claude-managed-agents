package memory

import (
	"fmt"
	"strings"
)

const MaxMountedStores = 8

// readOnlyStore wraps a MemoryStore and returns ErrForbidden for all write
// operations. It is used as a defence-in-depth measure for stores mounted with
// Writable=false: the tools layer already checks the writable flag, but this
// wrapper ensures that any code path that bypasses that check still fails safely.
type readOnlyStore struct {
	MemoryStore
}

func (r *readOnlyStore) Write(_, _, _ string) (string, error) { return "", ErrForbidden }
func (r *readOnlyStore) Edit(_, _, _, _ string) (string, error) { return "", ErrForbidden }
func (r *readOnlyStore) Delete(_ string) error               { return ErrForbidden }

// MountedStore pairs a MemoryStore with its write permission for this session.
type MountedStore struct {
	Store    MemoryStore
	Writable bool
}

// SessionStores is the set of MemoryStores mounted for a single session.
type SessionStores struct {
	mounted []*MountedStore
}

// NewSession creates a SessionStores with the three built-in stores.
// tenantWritable controls whether the tenant store accepts writes in this session.
func NewSession(user, project, tenant MemoryStore, tenantWritable bool) *SessionStores {
	tenantStore := wrapIfReadOnly(tenant, tenantWritable)
	return &SessionStores{
		mounted: []*MountedStore{
			{Store: user, Writable: true},
			{Store: project, Writable: true},
			{Store: tenantStore, Writable: tenantWritable},
		},
	}
}

// Mount adds a custom store to the session.
// Returns an error when the total number of mounted stores would exceed 8.
func (ss *SessionStores) Mount(store MemoryStore, writable bool) error {
	if len(ss.mounted) >= MaxMountedStores {
		return fmt.Errorf("memory: cannot mount more than %d stores per session", MaxMountedStores)
	}
	ss.mounted = append(ss.mounted, &MountedStore{Store: wrapIfReadOnly(store, writable), Writable: writable})
	return nil
}

func wrapIfReadOnly(store MemoryStore, writable bool) MemoryStore {
	if writable {
		return store
	}
	return &readOnlyStore{store}
}

// MountedStores returns all mounted stores.
func (ss *SessionStores) MountedStores() []*MountedStore {
	return ss.mounted
}

// FindStore locates a store by name.
// Returns the store, whether it was found, and whether writes are permitted.
func (ss *SessionStores) FindStore(name string) (MemoryStore, bool, bool) {
	for _, m := range ss.mounted {
		if m.Store.Name() == name {
			return m.Store, true, m.Writable
		}
	}
	return nil, false, false
}

// MergeList returns a formatted index of all mounted stores (memory_list output).
func (ss *SessionStores) MergeList() string {
	var sb strings.Builder
	for _, m := range ss.mounted {
		docs, err := m.Store.List()
		sb.WriteString("## ")
		sb.WriteString(m.Store.Name())
		sb.WriteString(" — ")
		sb.WriteString(m.Store.Description())
		sb.WriteByte('\n')
		if err != nil {
			sb.WriteString("(error loading documents)\n")
			sb.WriteByte('\n')
			continue
		}
		if len(docs) == 0 {
			sb.WriteString("(empty)\n")
			sb.WriteByte('\n')
			continue
		}
		for _, d := range docs {
			sb.WriteString("- [")
			sb.WriteString(d.Filename)
			sb.WriteString("] — ")
			sb.WriteString(d.Excerpt)
			sb.WriteByte('\n')
		}
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

// BuildSystemContext generates the Memory section injected into the system prompt.
// This is the single authoritative source of memory instructions; the static
// system prompt does not duplicate this content.
func (ss *SessionStores) BuildSystemContext() string {
	var sb strings.Builder
	sb.WriteString("# Memory\n\n")
	sb.WriteString("You have persistent memory across sessions. Use these tools:\n")
	sb.WriteString("- `memory_list` — index all documents across stores; call before starting a task\n")
	sb.WriteString("- `memory_read` — fetch a full document by filename\n")
	sb.WriteString("- `memory_write` — create or fully replace a document\n")
	sb.WriteString("- `memory_edit` — apply a targeted str-replace to an existing document\n")
	sb.WriteString("- `memory_search` — full-text search across stores\n")
	sb.WriteString("- `memory_delete` — remove a document no longer relevant\n\n")
	sb.WriteString("**Available stores:**\n")
	for _, m := range ss.mounted {
		sb.WriteString("- **")
		sb.WriteString(m.Store.Name())
		sb.WriteString("** — ")
		sb.WriteString(m.Store.Description())
		if !m.Writable {
			sb.WriteString(" (read-only)")
		}
		sb.WriteByte('\n')
	}
	sb.WriteString("\nChoose the store whose scope matches what you are recording.\n")
	return sb.String()
}
