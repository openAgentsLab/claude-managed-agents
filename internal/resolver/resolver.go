package resolver

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"

	"forge/internal/brain"
	"forge/internal/config"
	appstore "forge/internal/gateway/store"
)

// Resolver builds an EffectiveConfig for a given (tenantID, userID, agentID)
// and owns the tenant settings cache shared by all managers in the process.
// Configuration layers: tenant settings → user settings → agent config.
type Resolver struct {
	store      appstore.Store
	modelCache *modelCache

	settingsMu   sync.RWMutex
	settingsCache map[string]cachedSettings
}

type cachedSettings struct {
	ts        config.TenantSettings
	updatedAt time.Time
}

// New creates a Resolver.
func New(store appstore.Store) *Resolver {
	return &Resolver{
		store:         store,
		modelCache:    newModelCache(),
		settingsCache: make(map[string]cachedSettings),
	}
}

// SettingsFromStore converts the flat appstore.Settings (DB representation) to
// config.TenantSettings. Single authoritative conversion; replaces the duplicate
// tenantSettingsToConfig / storeTenantSettings functions.
func SettingsFromStore(s appstore.Settings) config.TenantSettings {
	ts := config.TenantSettings{
		AllowRules: s.AllowRules,
		DenyRules:  s.DenyRules,
		ResourceQuota: config.ResourceQuota{
			MemoryBytes: s.MemoryBytes,
			NanoCPUs:    s.NanoCPUs,
		},
	}
	if s.ModelOverride != nil {
		ts.ModelOverride = &config.ModelOverride{
			Provider:   s.ModelOverride.Provider,
			APIKey:     s.ModelOverride.APIKey,
			BaseURL:    s.ModelOverride.BaseURL,
			Model:      s.ModelOverride.Model,
			ByAzure:    s.ModelOverride.ByAzure,
			APIVersion: s.ModelOverride.APIVersion,
		}
	}
	if s.BrainOverride != nil {
		ts.BrainOverride = &config.BrainOverride{
			Effort:     s.BrainOverride.Effort,
			Thinking:   s.BrainOverride.Thinking,
			MaxRetries: s.BrainOverride.MaxRetries,
		}
	}
	return ts
}

// Settings returns cached TenantSettings for tenantID, loading from DB on first access.
func (r *Resolver) Settings(ctx context.Context, tenantID string) config.TenantSettings {
	r.settingsMu.RLock()
	entry, ok := r.settingsCache[tenantID]
	r.settingsMu.RUnlock()
	if ok {
		return entry.ts
	}
	// Cache miss: load from DB.
	t, err := r.store.Tenants().Get(ctx, tenantID)
	if err != nil {
		slog.WarnContext(ctx, "resolver: load tenant settings", "tenant", tenantID, "error", err)
		return config.TenantSettings{}
	}
	if t == nil {
		return config.TenantSettings{}
	}
	ts := SettingsFromStore(t.Settings)
	r.settingsMu.Lock()
	// Double-check: another goroutine may have populated the entry while we held no lock.
	if _, exists := r.settingsCache[tenantID]; !exists {
		r.settingsCache[tenantID] = cachedSettings{ts: ts, updatedAt: t.UpdatedAt}
	}
	r.settingsMu.Unlock()
	return ts
}

// PutSettings writes settings into the cache, replacing any existing entry.
// Call this after persisting settings to DB or when the refresh loop detects a change.
func (r *Resolver) PutSettings(tenantID string, ts config.TenantSettings, updatedAt time.Time) {
	r.settingsMu.Lock()
	r.settingsCache[tenantID] = cachedSettings{ts: ts, updatedAt: updatedAt}
	r.settingsMu.Unlock()
}

// SettingsUpdatedAt returns the DB updated_at timestamp of the cached settings entry,
// or zero time if the entry is not cached. Used by the refresh loop to detect staleness.
func (r *Resolver) SettingsUpdatedAt(tenantID string) time.Time {
	r.settingsMu.RLock()
	defer r.settingsMu.RUnlock()
	return r.settingsCache[tenantID].updatedAt
}

// Resolve builds EffectiveConfig for (tenantID, userID, agentID) by stacking:
//  1. Tenant settings: ModelOverride / BrainOverride (loaded from cache)
//  2. User settings: ModelOverride / BrainOverride (overrides tenant layer)
//  3. Agent config: model, system_prompt, tool_config (if agentID non-empty)
//  4. Agent MCP + Skill + callable-agent associations
//
// userID is the scoped internal ID ("tenantID/username"); pass empty string for
// worker brains that run outside a user session.
// When agentID is empty, only tenant+user settings are applied.
func (r *Resolver) Resolve(ctx context.Context, tenantID, userID, agentID string) (*EffectiveConfig, error) {
	eff := &EffectiveConfig{
		UserID:   userID,
		BrainCfg: brain.DefaultBrainConfig(),
	}

	applyTenantLayer(eff, r.Settings(ctx, tenantID))

	if userID != "" {
		username := strings.TrimPrefix(userID, tenantID+"/")
		us, err := r.store.Users().GetSettings(ctx, tenantID, username)
		if err != nil {
			slog.WarnContext(ctx, "resolver: load user settings", "user", userID, "error", err)
		} else {
			applyUserLayer(eff, us)
		}
	}

	if agentID == "" {
		return eff, nil
	}

	agent, err := r.store.Agents().Get(ctx, tenantID, agentID)
	if err != nil {
		slog.WarnContext(ctx, "resolver: get agent", "tenant", tenantID, "agent", agentID, "error", err)
		return eff, nil
	}
	if agent == nil {
		return eff, nil
	}

	if agent.Model != "" {
		eff.ModelCfg.Model = agent.Model
	}
	eff.BrainCfg.SystemPrompt = agent.SystemPrompt
	eff.ToolConfig = agent.ToolConfig

	if err := r.store.Agents().LoadAssociations(ctx, agent); err != nil {
		slog.WarnContext(ctx, "resolver: load agent associations", "agent", agentID, "error", err)
	}
	for _, name := range agent.MCPServerNames {
		rec, err := r.store.MCPServers().Get(ctx, tenantID, name)
		if err != nil || rec == nil || rec.Disabled {
			continue
		}
		eff.MCPRecs = append(eff.MCPRecs, rec)
	}
	for _, name := range agent.SkillNames {
		rec, err := r.store.UserSkills().Get(ctx, tenantID, "", name)
		if err != nil || rec == nil {
			continue
		}
		eff.SkillRecs = append(eff.SkillRecs, rec)
	}
	for _, cid := range agent.CallableAgents {
		ca, err := r.store.Agents().Get(ctx, tenantID, cid)
		if err != nil || ca == nil {
			continue
		}
		eff.CallableAgents = append(eff.CallableAgents, CallableAgentSummary{
			ID:          ca.ID,
			Name:        ca.Name,
			Description: ca.Description,
		})
	}

	return eff, nil
}

// ModelFor returns a cached model.ToolCallingChatModel for the effective config.
// Configs with the same ModelCacheKey share one HTTP client instance.
func (r *Resolver) ModelFor(ctx context.Context, eff *EffectiveConfig) (model.ToolCallingChatModel, error) {
	return r.modelCache.getOrCreate(ctx, eff.ModelCfg)
}
