package resolver

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"

	"forge/internal/brain"
	"forge/internal/gateway/vault"
	mcpclient "forge/internal/mcp/client"
	"forge/internal/skill"
	"forge/internal/skill/bundled"
	"forge/internal/tools"
)

const (
	workerBrainIdleTTL       = 5 * time.Minute
	workerBrainEvictInterval = 1 * time.Minute
)

// WorkerBrainEntry holds a Brain built for a worker task and its MCP cleanup.
// Key is the cache key used to release the entry via WorkerManager.ReleaseBrain.
// Brain.Run is goroutine-safe; multiple concurrent sub-agent threads may share the
// same WorkerBrainEntry safely.
type WorkerBrainEntry struct {
	Brain   *brain.Brain
	Key     string // cache key — pass to WorkerManager.ReleaseBrain when done
	cleanup func()
}

type workerCacheEntry struct {
	entry    *WorkerBrainEntry
	refs     int // active threads holding this entry; eviction waits until refs == 0
	lastUsed time.Time
}

// WorkerManager resolves per-tenant configuration and maintains a TTL-evicted
// Brain cache for worker goroutines. Unlike SessionManager (ref-counted,
// session-lifetime), WorkerManager uses pure idle-timeout eviction so that
// sequential tasks from the same tenant reuse the same Brain and MCP connections.
type WorkerManager struct {
	res       *Resolver
	globalReg tools.ToolRegistry
	secretRes vault.Resolver

	mu      sync.Mutex
	entries map[string]*workerCacheEntry // key = fingerprint+"|"+agentType
}

// NewWorkerManager creates a WorkerManager using the provided shared Resolver.
func NewWorkerManager(
	res *Resolver,
	globalReg tools.ToolRegistry,
	secretRes vault.Resolver,
) *WorkerManager {
	return &WorkerManager{
		res:       res,
		globalReg: globalReg,
		secretRes: secretRes,
		entries:   make(map[string]*workerCacheEntry),
	}
}

// AcquireBrain returns a cached or freshly-built Brain for (tenantID, agentID).
// Config is resolved via global→tenant→agent layers. Tool filtering is applied
// from the agent's ToolConfig. Cache key is the EffectiveConfig fingerprint so
// tasks sharing the same callable-agent config reuse the same brain instance.
//
// AcquireBrain increments the entry's ref count; callers MUST call ReleaseBrain
// with entry.Key when they are done to allow idle eviction.
//
// Brain construction happens outside the cache lock so parallel calls for
// different agents do not block each other. A double-check on re-entry handles
// the rare case where two goroutines build the same entry concurrently — the
// duplicate is discarded and the winner's entry is returned to both.
func (m *WorkerManager) AcquireBrain(
	ctx context.Context,
	tenantID, userID, agentID string,
) (*WorkerBrainEntry, error) {
	eff, err := m.res.Resolve(ctx, tenantID, userID, agentID)
	if err != nil {
		return nil, err
	}

	cacheKey := eff.Fingerprint() + "|worker|" + agentID

	// Fast path: already cached.
	m.mu.Lock()
	if e, ok := m.entries[cacheKey]; ok {
		e.refs++
		e.lastUsed = time.Now()
		m.mu.Unlock()
		return e.entry, nil
	}
	m.mu.Unlock()

	// Slow path: build outside the lock so concurrent requests for different
	// agents do not serialize unnecessarily.
	mdl, err := m.res.ModelFor(ctx, eff)
	if err != nil {
		return nil, err
	}
	allowTools, disallowTools := toolConfigToLists(eff.ToolConfig)
	newEntry, err := buildWorkerBrain(ctx, mdl, m.globalReg, eff, allowTools, disallowTools, m.secretRes, tenantID, userID)
	if err != nil {
		return nil, err
	}
	newEntry.Key = cacheKey

	// Double-check: another goroutine may have built and inserted an entry while
	// we were building. Use theirs and discard ours to avoid duplicate MCP connections.
	m.mu.Lock()
	if e, ok := m.entries[cacheKey]; ok {
		e.refs++
		e.lastUsed = time.Now()
		m.mu.Unlock()
		if newEntry.cleanup != nil {
			newEntry.cleanup()
		}
		return e.entry, nil
	}
	m.entries[cacheKey] = &workerCacheEntry{entry: newEntry, refs: 1, lastUsed: time.Now()}
	m.mu.Unlock()

	slog.DebugContext(ctx, "worker brain cache: built new entry", "key", cacheKey)
	return newEntry, nil
}

// ReleaseBrain decrements the ref count for the entry identified by key.
// When refs reach zero the entry becomes eligible for idle TTL eviction.
// Callers must pass entry.Key returned by AcquireBrain.
func (m *WorkerManager) ReleaseBrain(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[key]
	if !ok {
		return
	}
	e.refs--
}

// toolConfigToLists converts an agent ToolConfig map to allow/disallow slices
// for filterWorkerTools. Nil map means allow all tools.
func toolConfigToLists(tc map[string]bool) (allow, disallow []string) {
	for name, enabled := range tc {
		if enabled {
			allow = append(allow, name)
		} else {
			disallow = append(disallow, name)
		}
	}
	return
}

// StartEvictLoop runs TTL eviction until ctx is cancelled, then closes all
// remaining entries.
func (m *WorkerManager) StartEvictLoop(ctx context.Context) {
	ticker := time.NewTicker(workerBrainEvictInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			m.closeAll()
			return
		case <-ticker.C:
			m.evictIdle()
		}
	}
}

func (m *WorkerManager) evictIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, e := range m.entries {
		// Skip entries that are still held by active sub-agent threads.
		if e.refs > 0 {
			continue
		}
		if now.Sub(e.lastUsed) > workerBrainIdleTTL {
			if e.entry.cleanup != nil {
				e.entry.cleanup()
			}
			delete(m.entries, k)
			slog.Debug("worker brain cache: evicted idle entry", "key", k)
		}
	}
}

func (m *WorkerManager) closeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, e := range m.entries {
		if e.entry.cleanup != nil {
			e.entry.cleanup()
		}
		delete(m.entries, k)
	}
}

// buildWorkerBrain assembles MCP connections and skill tools from eff, combines
// them with the global registry, filters the result per the agent's allow/deny
// lists, and builds a Brain.
func buildWorkerBrain(
	ctx context.Context,
	m model.ToolCallingChatModel,
	globalReg tools.ToolRegistry,
	eff *EffectiveConfig,
	allowTools, disallowTools []string,
	secretRes vault.Resolver,
	tenantID string,
	userID string,
) (*WorkerBrainEntry, error) {

	var mcpMgr *mcpclient.Manager
	if len(eff.MCPRecs) > 0 {
		mcpMgr = mcpclient.NewManager()
		for _, r := range eff.MCPRecs {
			mcpMgr.Add(r.Name, mcpRecordToConfig(r, secretRes, ctx, tenantID, userID))
		}
		// Use context.Background(): worker brain connections outlive the task
		// request context that triggered construction.
		mcpMgr.ConnectAll(context.Background())
	}

	var userSkillReg *skill.Registry
	if len(eff.SkillRecs) > 0 {
		userSkillReg = skill.NewRegistry()
		bundled.Init(userSkillReg)
		for _, rec := range eff.SkillRecs {
			fm, body, err := skill.ParseSkillFile(rec.Content)
			if err != nil {
				slog.WarnContext(ctx, "worker brain: skip unparseable skill",
					"name", rec.Name, "tenant", tenantID, "error", err)
				continue
			}
			userSkillReg.RegisterDynamic(&skill.Skill{
				Frontmatter: fm,
				Content:     body,
				Source:      skill.SourceDynamic,
			})
		}
	}

	allTools := assembleTools(ctx, globalReg, mcpMgr, userSkillReg)
	filtered := filterWorkerTools(ctx, allTools, allowTools, disallowTools)

	b, err := brain.New(ctx, m, tools.Static(filtered), eff.BrainCfg)
	if err != nil {
		if mcpMgr != nil {
			mcpMgr.Close()
		}
		return nil, err
	}

	var cleanup func()
	if mcpMgr != nil {
		cleanup = mcpMgr.Close
	}
	return &WorkerBrainEntry{Brain: b, cleanup: cleanup}, nil
}

// filterWorkerTools filters a tool slice by allow and disallow lists.
// allow == nil or ["*"] means start with all tools; an explicit list is an
// allowlist. disallow is always applied after, removing matching names.
func filterWorkerTools(ctx context.Context, all []einotool.BaseTool, allow, disallow []string) []einotool.BaseTool {
	allWildcard := len(allow) == 0 || (len(allow) == 1 && allow[0] == "*")

	allowSet := make(map[string]bool, len(allow))
	for _, name := range allow {
		allowSet[name] = true
	}
	disallowSet := make(map[string]bool, len(disallow))
	for _, name := range disallow {
		disallowSet[name] = true
	}

	out := make([]einotool.BaseTool, 0, len(all))
	for _, t := range all {
		info, err := t.Info(ctx)
		if err != nil || info == nil {
			continue
		}
		name := info.Name
		if disallowSet[name] {
			continue
		}
		if allWildcard || allowSet[name] {
			out = append(out, t)
		}
	}
	return out
}
