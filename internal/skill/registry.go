package skill

import (
	"log/slog"
	"sync"
)

// Registry holds all loaded skills and provides thread-safe lookup.
type Registry struct {
	mu     sync.RWMutex
	active map[string]*Skill
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		active: make(map[string]*Skill),
	}
}

// RegisterBundled adds a bundled skill. Bundled skills are only inserted when
// no skill with the same name is already present, so filesystem skills win.
func (r *Registry) RegisterBundled(s *Skill) {
	name := s.Name()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.active[name]; !exists {
		r.active[name] = s
		for _, alias := range s.Frontmatter.Aliases {
			if _, exists := r.active[alias]; !exists {
				r.active[alias] = s
			}
		}
		slog.Debug("skill: registered bundled", "name", name)
	}
}

// RegisterDynamic adds a DB-sourced skill with the highest priority — it
// unconditionally overwrites any previously registered skill with the same name
// (bundled or otherwise). Intended for user-configured skills loaded from
// the serve-mode store at session creation time.
func (r *Registry) RegisterDynamic(s *Skill) {
	name := s.Name()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active[name] = s
	for _, alias := range s.Frontmatter.Aliases {
		r.active[alias] = s
	}
	slog.Debug("skill: registered dynamic", "name", name)
}

// Find returns the skill with the given name (or alias).
func (r *Registry) Find(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.active[name]
	return s, ok
}

// All returns all currently active skills, deduplicated by pointer.
func (r *Registry) All() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[*Skill]bool, len(r.active))
	var out []*Skill
	for _, s := range r.active {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
