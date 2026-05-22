package resolver

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"

	"forge/internal/brain"
	"forge/internal/gateway/session"
	appstore "forge/internal/gateway/store"
	"forge/internal/gateway/vault"
	"forge/internal/history"
	mcpclient "forge/internal/mcp/client"
	"forge/internal/skill"
	"forge/internal/skill/bundled"
	"forge/internal/subagent"
	"forge/internal/tools"
	mcptools "forge/internal/tools/mcp"
	skilltool "forge/internal/tools/skill"
)

const (
	brainCacheIdleTTL       = 30 * time.Minute
	brainCacheEvictInterval = 5 * time.Minute
)

// SessionBrainEntry holds a per-session Brain, its history manager, and cleanup.
type SessionBrainEntry struct {
	Brain   *brain.Brain
	History history.Manager
	cleanup func()
}

// ── brain cache ───────────────────────────────────────────────────────────────

type brainCache struct {
	mu      sync.Mutex
	entries map[string]*cachedBrainEntry
}

type cachedBrainEntry struct {
	sbEntry  *SessionBrainEntry
	refs     int
	lastUsed time.Time
}

func newBrainCache() *brainCache {
	return &brainCache{entries: make(map[string]*cachedBrainEntry)}
}

func (c *brainCache) acquire(key string, build func() (*SessionBrainEntry, error)) (*SessionBrainEntry, error) {
	// Fast path: already cached.
	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		e.refs++
		e.lastUsed = time.Now()
		c.mu.Unlock()
		return e.sbEntry, nil
	}
	c.mu.Unlock()

	// Slow path: build outside the lock so concurrent requests for different
	// configurations do not serialize behind a single mutex.
	entry, err := build()
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	// Double-check: another goroutine may have inserted the same entry while we
	// were building. Use theirs and discard ours to avoid duplicate MCP connections.
	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		e.refs++
		e.lastUsed = time.Now()
		c.mu.Unlock()
		if entry.cleanup != nil {
			entry.cleanup()
		}
		return e.sbEntry, nil
	}
	c.entries[key] = &cachedBrainEntry{sbEntry: entry, refs: 1, lastUsed: time.Now()}
	c.mu.Unlock()
	return entry, nil
}

func (c *brainCache) release(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[key]
	if !ok {
		return
	}
	e.refs--
	if e.refs <= 0 {
		if e.sbEntry.cleanup != nil {
			e.sbEntry.cleanup()
		}
		delete(c.entries, key)
	}
}

// evictIdle removes idle entries from the cache and returns their fingerprints
// plus a deferred cleanup function. The caller must update sessionBrains/sessionKeys
// BEFORE invoking runCleanup so that concurrent Ensure() calls cannot return a
// brain entry whose MCP connections are about to be closed.
func (c *brainCache) evictIdle(ttl time.Duration) (evicted []string, runCleanup func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var cleanups []func()
	now := time.Now()
	for key, e := range c.entries {
		if e.refs > 0 {
			continue // still held by active sessions; never evict a live brain
		}
		if now.Sub(e.lastUsed) > ttl {
			evicted = append(evicted, key)
			if e.sbEntry.cleanup != nil {
				cleanups = append(cleanups, e.sbEntry.cleanup)
			}
			delete(c.entries, key)
		}
	}
	return evicted, func() {
		for _, fn := range cleanups {
			fn()
		}
	}
}

// ── SessionManager ────────────────────────────────────────────────────────────

// SessionManager owns the full per-session brain lifecycle: config resolution,
// model caching, brain construction, and ref-counted brain caching.
type SessionManager struct {
	res           *Resolver
	brainCache    *brainCache
	sessionBrains sync.Map // internalSID → *SessionBrainEntry
	sessionKeys   sync.Map // internalSID → fingerprint
	globalReg     tools.ToolRegistry
	secretRes     vault.Resolver
	workerMgr     *WorkerManager
	sessionStore  session.SessionStore
}

// NewSessionManager creates a SessionManager using the provided shared Resolver.
func NewSessionManager(
	res *Resolver,
	workerMgr *WorkerManager,
	sessionStore session.SessionStore,
	globalReg tools.ToolRegistry,
	secretRes vault.Resolver,
) *SessionManager {
	return &SessionManager{
		res:          res,
		brainCache:   newBrainCache(),
		globalReg:    globalReg,
		secretRes:    secretRes,
		workerMgr:    workerMgr,
		sessionStore: sessionStore,
	}
}

// Ensure returns the SessionBrainEntry for internalSID, building it lazily on
// first access. Returns nil on error; the caller must treat nil as a fatal
// condition — there is no global brain fallback.
func (m *SessionManager) Ensure(
	ctx context.Context,
	internalSID, tenantID, userID, agentID string,
) *SessionBrainEntry {
	if v, ok := m.sessionBrains.Load(internalSID); ok {
		return v.(*SessionBrainEntry)
	}

	eff, err := m.res.Resolve(ctx, tenantID, userID, agentID)
	if err != nil {
		slog.ErrorContext(ctx, "session brain: resolver failed", "error", err)
		return nil
	}
	mdl, err := m.res.ModelFor(ctx, eff)
	if err != nil {
		slog.ErrorContext(ctx, "session brain: model init failed", "error", err)
		return nil
	}
	fp := eff.Fingerprint()
	entry, err := m.brainCache.acquire(fp, func() (*SessionBrainEntry, error) {
		return buildSessionBrain(ctx, mdl, m.globalReg, eff, m.secretRes, m.workerMgr, m.sessionStore, tenantID, userID)
	})
	if err != nil {
		slog.ErrorContext(ctx, "session brain: build failed", "error", err)
		return nil
	}
	if _, loaded := m.sessionBrains.LoadOrStore(internalSID, entry); loaded {
		m.brainCache.release(fp)
	} else {
		m.sessionKeys.Store(internalSID, fp)
	}
	return entry
}

// Release decrements the ref count for internalSID's brain. When it reaches
// zero, MCP connections are closed.
func (m *SessionManager) Release(internalSID string) {
	if fp, ok := m.sessionKeys.LoadAndDelete(internalSID); ok {
		m.brainCache.release(fp.(string))
		m.sessionBrains.Delete(internalSID)
	}
}

// StartEvictLoop runs a background goroutine that evicts idle brain cache
// entries. Blocks until ctx is cancelled; call in a goroutine.
func (m *SessionManager) StartEvictLoop(ctx context.Context) {
	ticker := time.NewTicker(brainCacheEvictInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.evictIdle()
		}
	}
}

func (m *SessionManager) evictIdle() {
	evicted, runCleanup := m.brainCache.evictIdle(brainCacheIdleTTL)
	if len(evicted) == 0 {
		return
	}
	evictedFPs := make(map[string]struct{}, len(evicted))
	for _, fp := range evicted {
		evictedFPs[fp] = struct{}{}
	}
	// Remove from sessionBrains BEFORE running MCP cleanup so that any
	// concurrent Ensure() call cannot retrieve a brain entry that is about
	// to have its connections closed.
	m.sessionKeys.Range(func(k, v any) bool {
		if _, ok := evictedFPs[v.(string)]; ok {
			m.sessionBrains.Delete(k)
			m.sessionKeys.Delete(k)
		}
		return true
	})
	runCleanup()
}

// ── session brain construction ────────────────────────────────────────────────

func buildSessionBrain(
	ctx context.Context,
	m model.ToolCallingChatModel,
	globalReg tools.ToolRegistry,
	eff *EffectiveConfig,
	secretRes vault.Resolver,
	workerMgr *WorkerManager,
	sessionStore session.SessionStore,
	tenantID string,
	userID string,
) (*SessionBrainEntry, error) {

	var mcpMgr *mcpclient.Manager
	if len(eff.MCPRecs) > 0 {
		mcpMgr = mcpclient.NewManager()
		for _, r := range eff.MCPRecs {
			mcpMgr.Add(r.Name, mcpRecordToConfig(r, secretRes, ctx, tenantID, userID))
		}
		// Use a background context: MCP connections outlive the HTTP request that
		// triggered session creation. Using the request ctx would tear down the
		// connections as soon as the request completes.
		mcpMgr.ConnectAll(context.Background())
		slog.InfoContext(ctx, "session brain: MCP connected", "tenant", tenantID, "servers", len(eff.MCPRecs))
	}

	var userSkillReg *skill.Registry
	if len(eff.SkillRecs) > 0 {
		userSkillReg = skill.NewRegistry()
		bundled.Init(userSkillReg)
		for _, rec := range eff.SkillRecs {
			fm, body, err := skill.ParseSkillFile(rec.Content)
			if err != nil {
				slog.WarnContext(ctx, "session brain: skip unparseable skill",
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

	sessionTools := assembleTools(ctx, globalReg, mcpMgr, userSkillReg)

	// Copy BrainCfg before mutating so that eff (owned by the caller) is not
	// modified. This prevents pollution of any cached EffectiveConfig and keeps
	// buildSessionBrain a pure function with respect to its inputs.
	brainCfg := eff.BrainCfg

	// Inject per-session dispatch_agent_task tool when callable agents are configured.
	// This tool is coordinator-only: workers use buildWorkerBrain which never receives it.
	if workerMgr != nil && len(eff.CallableAgents) > 0 {
		callables := make([]subagent.CallableAgent, len(eff.CallableAgents))
		for i, ca := range eff.CallableAgents {
			callables[i] = subagent.CallableAgent{ID: ca.ID, Name: ca.Name, Description: ca.Description}
		}
		acquireBrain := func(ctx context.Context, tenantID, agentID string) (*brain.Brain, func(), error) {
			entry, err := workerMgr.AcquireBrain(ctx, tenantID, userID, agentID)
			if err != nil {
				return nil, func() {}, err
			}
			release := func() { workerMgr.ReleaseBrain(entry.Key) }
			return entry.Brain, release, nil
		}
		sessionTools = append(sessionTools, subagent.NewAgentTool(callables, acquireBrain, sessionStore))
		brainCfg.SystemPrompt = appendCallableAgentContext(brainCfg.SystemPrompt, eff.CallableAgents)
	}

	b, err := brain.New(ctx, m, tools.Static(sessionTools), brainCfg)
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
	hist := history.BuildWithDefaultCompact(sessionStore, m)
	return &SessionBrainEntry{Brain: b, History: hist, cleanup: cleanup}, nil
}

func assembleTools(
	ctx context.Context,
	globalReg tools.ToolRegistry,
	mcpMgr *mcpclient.Manager,
	userSkillReg *skill.Registry,
) []einotool.BaseTool {
	var out []einotool.BaseTool
	for _, t := range globalReg.Tools() {
		if userSkillReg != nil {
			info, err := t.Info(ctx)
			if err == nil && info != nil && info.Name == "use_skill" {
				continue
			}
		}
		out = append(out, t)
	}
	if userSkillReg != nil {
		out = append(out, skilltool.New(userSkillReg))
	}
	if mcpMgr != nil {
		out = append(out, mcptools.NewRegistryFromManager(mcpMgr).Tools()...)
	}
	return out
}


// mcpRecordToConfig converts a stored MCPServerRecord to a live config, resolving
// any vault references in Env and Headers. Resolution uses r.UserID so that:
//   - tenant-level records (r.UserID="") resolve against the tenant vault only
// vault refs in Env/Headers are resolved against the tenant vault.
func mcpRecordToConfig(r *appstore.MCPServerRecord, res vault.Resolver, ctx context.Context, tenantID, userID string) mcpclient.MCPServerConfig {
	return mcpclient.MCPServerConfig{
		Type:     mcpclient.MCPServerType(r.Type),
		Disabled: r.Disabled,
		Command:  r.Command,
		Args:     r.Args,
		Env:      resolveMapRefs(ctx, r.Env, res, tenantID, userID),
		URL:      r.URL,
		Headers:  resolveMapRefs(ctx, r.Headers, res, tenantID, userID),
	}
}

func appendCallableAgentContext(sysPrompt string, agents []CallableAgentSummary) string {
	var sb strings.Builder
	sb.WriteString(sysPrompt)
	sb.WriteString("\n\n# Sub-agents\n\n")
	sb.WriteString("Use `dispatch_agent_task` to delegate independent work to a specialized sub-agent.\n")
	sb.WriteString("The sub-agent runs synchronously and returns its result before you continue.\n")
	sb.WriteString("Multiple calls in the same turn execute concurrently.\n\n")
	sb.WriteString("**Available agents:**\n")
	for _, a := range agents {
		sb.WriteString("- **")
		sb.WriteString(a.Name)
		sb.WriteString("** (`")
		sb.WriteString(a.ID)
		sb.WriteString("`) — ")
		sb.WriteString(a.Description)
		sb.WriteByte('\n')
	}
	sb.WriteString("\nDelegate when the work is truly independent of the current turn.\n")
	sb.WriteString("Do not delegate tasks that depend on in-progress results.\n")
	return sb.String()
}

func resolveMapRefs(ctx context.Context, m map[string]string, res vault.Resolver, tenantID, userID string) map[string]string {
	if len(m) == 0 || res == nil {
		return m
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		resolved, err := res.Resolve(ctx, tenantID, userID, v)
		if err != nil {
			slog.WarnContext(ctx, "session brain: vault resolve failed",
				"key", k, "user", userID, "error", err)
			continue
		}
		out[k] = resolved
	}
	return out
}
