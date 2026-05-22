package resolver

import (
	"testing"
	"time"

	"forge/internal/brain"
	"forge/internal/config"
	appstore "forge/internal/gateway/store"
)

// ── ModelCacheKey ─────────────────────────────────────────────────────────────

func TestModelCacheKey_Deterministic(t *testing.T) {
	cfg := config.ModelConfig{Provider: "anthropic", APIKey: "key", Model: "claude-haiku-4-5-20251001"}
	k1 := ModelCacheKey(cfg)
	k2 := ModelCacheKey(cfg)
	if k1 != k2 {
		t.Errorf("ModelCacheKey must be deterministic; got %q and %q", k1, k2)
	}
}

func TestModelCacheKey_DifferentConfigs(t *testing.T) {
	a := config.ModelConfig{Provider: "anthropic", Model: "haiku"}
	b := config.ModelConfig{Provider: "openai", Model: "gpt-4o"}
	if ModelCacheKey(a) == ModelCacheKey(b) {
		t.Error("different ModelConfigs should produce different cache keys")
	}
}

func TestModelCacheKey_APIKeyContributes(t *testing.T) {
	base := config.ModelConfig{Provider: "anthropic", Model: "haiku"}
	withKey := config.ModelConfig{Provider: "anthropic", Model: "haiku", APIKey: "sk-secret"}
	if ModelCacheKey(base) == ModelCacheKey(withKey) {
		t.Error("different API keys should produce different cache keys")
	}
}

// ── EffectiveConfig.Fingerprint ───────────────────────────────────────────────

func TestFingerprint_Deterministic(t *testing.T) {
	e := &EffectiveConfig{
		UserID:   "tenant1/alice",
		ModelCfg: config.ModelConfig{Provider: "anthropic", Model: "haiku", APIKey: "k"},
		BrainCfg: brain.BrainConfig{SystemPrompt: "You are helpful."},
	}
	if e.Fingerprint() != e.Fingerprint() {
		t.Error("Fingerprint must be deterministic")
	}
}

func TestFingerprint_DifferentUsers(t *testing.T) {
	a := &EffectiveConfig{
		UserID:   "tenant1/alice",
		ModelCfg: config.ModelConfig{Provider: "anthropic", Model: "haiku", APIKey: "k"},
	}
	b := &EffectiveConfig{
		UserID:   "tenant1/bob",
		ModelCfg: config.ModelConfig{Provider: "anthropic", Model: "haiku", APIKey: "k"},
	}
	if a.Fingerprint() == b.Fingerprint() {
		t.Error("different user IDs should produce different fingerprints")
	}
}

func TestFingerprint_DifferentModels(t *testing.T) {
	a := &EffectiveConfig{ModelCfg: config.ModelConfig{Provider: "anthropic", Model: "haiku"}}
	b := &EffectiveConfig{ModelCfg: config.ModelConfig{Provider: "anthropic", Model: "sonnet"}}
	if a.Fingerprint() == b.Fingerprint() {
		t.Error("different models should produce different fingerprints")
	}
}

func TestFingerprint_DifferentSystemPrompts(t *testing.T) {
	a := &EffectiveConfig{BrainCfg: brain.BrainConfig{SystemPrompt: "You are X."}}
	b := &EffectiveConfig{BrainCfg: brain.BrainConfig{SystemPrompt: "You are Y."}}
	if a.Fingerprint() == b.Fingerprint() {
		t.Error("different system prompts should produce different fingerprints")
	}
}

func TestFingerprint_MCPOrderIndependent(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := &EffectiveConfig{
		MCPRecs: []*appstore.MCPServerRecord{
			{Name: "fs", UpdatedAt: ts},
			{Name: "git", UpdatedAt: ts},
		},
	}
	b := &EffectiveConfig{
		MCPRecs: []*appstore.MCPServerRecord{
			{Name: "git", UpdatedAt: ts},
			{Name: "fs", UpdatedAt: ts},
		},
	}
	if a.Fingerprint() != b.Fingerprint() {
		t.Error("MCP server order should not affect fingerprint")
	}
}

func TestFingerprint_MCPTimestampChanges(t *testing.T) {
	ts1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	a := &EffectiveConfig{
		MCPRecs: []*appstore.MCPServerRecord{{Name: "fs", UpdatedAt: ts1}},
	}
	b := &EffectiveConfig{
		MCPRecs: []*appstore.MCPServerRecord{{Name: "fs", UpdatedAt: ts2}},
	}
	if a.Fingerprint() == b.Fingerprint() {
		t.Error("updated MCP server timestamp should change fingerprint")
	}
}

func TestFingerprint_CallableAgentsOrderIndependent(t *testing.T) {
	a := &EffectiveConfig{
		CallableAgents: []CallableAgentSummary{
			{ID: "a1"}, {ID: "a2"},
		},
	}
	b := &EffectiveConfig{
		CallableAgents: []CallableAgentSummary{
			{ID: "a2"}, {ID: "a1"},
		},
	}
	if a.Fingerprint() != b.Fingerprint() {
		t.Error("callable agent order should not affect fingerprint")
	}
}

// ── mergeModelCfg ─────────────────────────────────────────────────────────────

func TestMergeModelCfg_OverridesNonEmpty(t *testing.T) {
	base := config.ModelConfig{Provider: "anthropic", Model: "haiku", APIKey: "base-key"}
	ov := &config.ModelOverride{Model: "sonnet"}
	mergeModelCfg(&base, ov)
	if base.Model != "sonnet" {
		t.Errorf("Model: got %q, want %q", base.Model, "sonnet")
	}
	if base.Provider != "anthropic" {
		t.Error("Provider should be unchanged when override is empty")
	}
	if base.APIKey != "base-key" {
		t.Error("APIKey should be unchanged when override is empty")
	}
}

func TestMergeModelCfg_EmptyOverrideNoOp(t *testing.T) {
	base := config.ModelConfig{Provider: "openai", Model: "gpt-4o"}
	ov := &config.ModelOverride{}
	mergeModelCfg(&base, ov)
	if base.Provider != "openai" || base.Model != "gpt-4o" {
		t.Errorf("empty override should be no-op; got %+v", base)
	}
}

func TestMergeModelCfg_AzureFlag(t *testing.T) {
	base := config.ModelConfig{}
	ov := &config.ModelOverride{ByAzure: true, APIVersion: "2024-02"}
	mergeModelCfg(&base, ov)
	if !base.ByAzure {
		t.Error("ByAzure flag should be propagated")
	}
	if base.APIVersion != "2024-02" {
		t.Errorf("APIVersion: got %q", base.APIVersion)
	}
}

// ── mergeBrainCfg ─────────────────────────────────────────────────────────────

func TestMergeBrainCfg_OverridesEffort(t *testing.T) {
	base := brain.BrainConfig{Effort: "normal"}
	ov := &config.BrainOverride{Effort: "high"}
	mergeBrainCfg(&base, ov)
	if base.Effort != "high" {
		t.Errorf("Effort: got %q, want %q", base.Effort, "high")
	}
}

func TestMergeBrainCfg_OverridesThinking(t *testing.T) {
	base := brain.BrainConfig{}
	ov := &config.BrainOverride{Thinking: "enabled"}
	mergeBrainCfg(&base, ov)
	if base.Thinking.Type != "enabled" {
		t.Errorf("Thinking.Type: got %q", base.Thinking.Type)
	}
}

func TestMergeBrainCfg_OverridesMaxRetries(t *testing.T) {
	base := brain.BrainConfig{MaxRetries: 2}
	ov := &config.BrainOverride{MaxRetries: 5}
	mergeBrainCfg(&base, ov)
	if base.MaxRetries != 5 {
		t.Errorf("MaxRetries: got %d", base.MaxRetries)
	}
}

func TestMergeBrainCfg_ZeroMaxRetriesNoOp(t *testing.T) {
	base := brain.BrainConfig{MaxRetries: 3}
	ov := &config.BrainOverride{MaxRetries: 0}
	mergeBrainCfg(&base, ov)
	if base.MaxRetries != 3 {
		t.Errorf("zero MaxRetries override should be no-op; got %d", base.MaxRetries)
	}
}
