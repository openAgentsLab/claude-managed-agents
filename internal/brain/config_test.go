package brain

import (
	"testing"
)

func TestDefaultBrainConfig_Effort(t *testing.T) {
	cfg := DefaultBrainConfig()
	if cfg.Effort != "high" {
		t.Errorf("Effort = %q, want \"high\"", cfg.Effort)
	}
}

func TestDefaultBrainConfig_ThinkingAdaptive(t *testing.T) {
	cfg := DefaultBrainConfig()
	if cfg.Thinking.Type != "adaptive" {
		t.Errorf("Thinking.Type = %q, want \"adaptive\"", cfg.Thinking.Type)
	}
}

func TestDefaultBrainConfig_MaxRetries(t *testing.T) {
	cfg := DefaultBrainConfig()
	if cfg.MaxRetries <= 0 {
		t.Errorf("MaxRetries = %d, want > 0", cfg.MaxRetries)
	}
}

func TestBrainConfig_ZeroValueIsDistinctFromDefault(t *testing.T) {
	var zero BrainConfig
	def := DefaultBrainConfig()
	if zero.Effort == def.Effort {
		t.Error("zero-value config should have empty Effort; DefaultBrainConfig must populate it")
	}
}

func TestBrainConfig_MaxIterations_Default(t *testing.T) {
	cfg := DefaultBrainConfig()
	if cfg.maxIterations() != 48 {
		t.Errorf("default MaxIterations = %d, want 48", cfg.maxIterations())
	}
}

func TestBrainConfig_MaxIterations_Zero(t *testing.T) {
	cfg := BrainConfig{MaxIterations: 0}
	if cfg.maxIterations() != 48 {
		t.Errorf("zero MaxIterations should default to 48, got %d", cfg.maxIterations())
	}
}

func TestBrainConfig_MaxIterations_Custom(t *testing.T) {
	cfg := BrainConfig{MaxIterations: 10}
	if cfg.maxIterations() != 10 {
		t.Errorf("custom MaxIterations should be 10, got %d", cfg.maxIterations())
	}
}
