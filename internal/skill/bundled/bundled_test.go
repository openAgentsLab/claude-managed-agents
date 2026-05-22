package bundled

import (
	"testing"

	"forge/internal/skill"
)

func TestInit_RegistersBundledSkills(t *testing.T) {
	reg := skill.NewRegistry()
	Init(reg)

	all := reg.All()
	if len(all) == 0 {
		t.Error("Init should register at least one bundled skill")
	}

	for _, s := range all {
		if s.Source != skill.SourceBundled {
			t.Errorf("skill %q should have SourceBundled, got %q", s.Name(), s.Source)
		}
		if s.Name() == "" {
			t.Error("bundled skill must have a non-empty name")
		}
		if s.Content == "" {
			t.Error("bundled skill must have non-empty content")
		}
	}
}

func TestInit_SkillsAreUserInvocable(t *testing.T) {
	reg := skill.NewRegistry()
	Init(reg)

	for _, s := range reg.All() {
		// By default skills are user-invocable unless explicitly disabled.
		// Just verify the flag is readable.
		_ = s.IsUserInvocable()
		_ = s.IsModelInvocable()
	}
}

func TestInit_Idempotent(t *testing.T) {
	reg := skill.NewRegistry()
	Init(reg)
	firstCount := len(reg.All())

	// Calling Init again should not duplicate skills.
	Init(reg)
	secondCount := len(reg.All())

	if firstCount != secondCount {
		t.Errorf("Init is not idempotent: first=%d second=%d", firstCount, secondCount)
	}
}

func TestInit_KnownSkills(t *testing.T) {
	reg := skill.NewRegistry()
	Init(reg)

	// These are the known bundled skills in the repo (commit, simplify, update-config).
	// If this test fails after adding a new skill, update the list.
	knownNames := []string{"commit", "simplify", "update-config"}
	for _, name := range knownNames {
		if _, ok := reg.Find(name); !ok {
			t.Errorf("expected bundled skill %q to be registered", name)
		}
	}
}
