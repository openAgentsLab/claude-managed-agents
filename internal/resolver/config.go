// Package resolver stacks global → tenant → user configuration layers and
// produces an EffectiveConfig ready for session-brain construction.
//
// Dependency direction: resolver → brain (config types), config, appstore.
// Nothing in brain/ or orchestration/ should be imported here.
package resolver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"time"

	"forge/internal/brain"
	"forge/internal/config"
	appstore "forge/internal/gateway/store"
)

// CallableAgentSummary is the minimal info needed by dispatch_agent_task to
// describe and validate callable agents in the tool schema.
type CallableAgentSummary struct {
	ID          string
	Name        string
	Description string
}

// EffectiveConfig is the fully-resolved per-session configuration produced by
// stacking tenant → user → agent layers. Downstream code (session brain
// construction, permission engine lookup) consumes this struct exclusively and
// never reads raw config layers directly.
type EffectiveConfig struct {
	UserID         string // scoped user ID ("tenantID/username"); empty for worker brains
	ModelCfg       config.ModelConfig
	BrainCfg       brain.BrainConfig
	MCPRecs        []*appstore.MCPServerRecord
	SkillRecs      []*appstore.UserSkillRecord
	ToolConfig     map[string]bool        // agent-level tool enable/disable
	CallableAgents []CallableAgentSummary // agents this session's brain may dispatch tasks to
}

// Fingerprint returns a stable hash that uniquely identifies the brain
// configuration implied by this EffectiveConfig. Two EffectiveConfigs with the
// same Fingerprint can safely share one Brain instance and its MCP connections.
//
// The fingerprint covers model identity and the UpdatedAt timestamps of active
// MCP and skill records — if a record is updated in the DB, its UpdatedAt
// advances and the fingerprint changes, triggering a rebuild.
func (e *EffectiveConfig) Fingerprint() string {
	h := sha256.New()
	io.WriteString(h, "v2\n")
	io.WriteString(h, "user|"+e.UserID+"\n")
	io.WriteString(h, ModelCacheKey(e.ModelCfg)+"\n")
	fmt.Fprintf(h, "sysprompt|%x\n", sha256.Sum256([]byte(e.BrainCfg.SystemPrompt)))

	type entry struct{ name, ts string }

	mcpEntries := make([]entry, 0, len(e.MCPRecs))
	for _, r := range e.MCPRecs {
		mcpEntries = append(mcpEntries, entry{r.Name, r.UpdatedAt.Format(time.RFC3339Nano)})
	}
	sort.Slice(mcpEntries, func(i, j int) bool { return mcpEntries[i].name < mcpEntries[j].name })
	for _, en := range mcpEntries {
		fmt.Fprintf(h, "mcp|%s|%s\n", en.name, en.ts)
	}

	skillEntries := make([]entry, 0, len(e.SkillRecs))
	for _, r := range e.SkillRecs {
		skillEntries = append(skillEntries, entry{r.Name, r.UpdatedAt.Format(time.RFC3339Nano)})
	}
	sort.Slice(skillEntries, func(i, j int) bool { return skillEntries[i].name < skillEntries[j].name })
	for _, en := range skillEntries {
		fmt.Fprintf(h, "skill|%s|%s\n", en.name, en.ts)
	}

	callableIDs := make([]string, 0, len(e.CallableAgents))
	for _, ca := range e.CallableAgents {
		callableIDs = append(callableIDs, ca.ID)
	}
	sort.Strings(callableIDs)
	for _, id := range callableIDs {
		fmt.Fprintf(h, "callable|%s\n", id)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// ModelCacheKey returns a string that uniquely identifies a model configuration.
// Exported so callers can derive cache keys without duplicating the logic.
func ModelCacheKey(c config.ModelConfig) string {
	return c.Provider + "|" + c.APIKey + "|" + c.BaseURL + "|" + c.Model
}

// applyTenantLayer overlays tenant-level model and brain overrides onto eff.
func applyTenantLayer(eff *EffectiveConfig, ts config.TenantSettings) {
	if ts.ModelOverride != nil {
		mergeModelCfg(&eff.ModelCfg, ts.ModelOverride)
	}
	if ts.BrainOverride != nil {
		mergeBrainCfg(&eff.BrainCfg, ts.BrainOverride)
	}
}

// applyUserLayer overlays user-level model and brain overrides onto eff,
// taking precedence over the tenant layer already applied.
func applyUserLayer(eff *EffectiveConfig, us appstore.UserSettings) {
	if us.ModelOverride != nil {
		mergeModelCfg(&eff.ModelCfg, &config.ModelOverride{
			Provider:   us.ModelOverride.Provider,
			APIKey:     us.ModelOverride.APIKey,
			BaseURL:    us.ModelOverride.BaseURL,
			Model:      us.ModelOverride.Model,
			ByAzure:    us.ModelOverride.ByAzure,
			APIVersion: us.ModelOverride.APIVersion,
		})
	}
	if us.BrainOverride != nil {
		mergeBrainCfg(&eff.BrainCfg, &config.BrainOverride{
			Effort:     us.BrainOverride.Effort,
			Thinking:   us.BrainOverride.Thinking,
			MaxRetries: us.BrainOverride.MaxRetries,
		})
	}
}


func mergeModelCfg(base *config.ModelConfig, ov *config.ModelOverride) {
	if ov.Provider != "" {
		base.Provider = ov.Provider
	}
	if ov.APIKey != "" {
		base.APIKey = ov.APIKey
	}
	if ov.BaseURL != "" {
		base.BaseURL = ov.BaseURL
	}
	if ov.Model != "" {
		base.Model = ov.Model
	}
	if ov.ByAzure {
		base.ByAzure = true
	}
	if ov.APIVersion != "" {
		base.APIVersion = ov.APIVersion
	}
}

func mergeBrainCfg(base *brain.BrainConfig, ov *config.BrainOverride) {
	if ov.Effort != "" {
		base.Effort = ov.Effort
	}
	if ov.Thinking != "" {
		base.Thinking = brain.ThinkingConfig{Type: ov.Thinking}
	}
	if ov.MaxRetries > 0 {
		base.MaxRetries = ov.MaxRetries
	}
}
