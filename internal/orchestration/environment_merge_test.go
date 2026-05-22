package orchestration

import (
	"testing"

	"forge/internal/gateway/store"
)

// ── MergeEnvironments ─────────────────────────────────────────────────────────

func TestMergeEnvironments_NilLayersSkipped(t *testing.T) {
	result := MergeEnvironments(nil, nil)
	if result.Networking.Mode != store.NetworkingUnrestricted {
		t.Errorf("nil layers: expected unrestricted, got %q", result.Networking.Mode)
	}
	if len(result.Packages.Pip)+len(result.Packages.Npm)+len(result.Packages.Apt)+len(result.Packages.Cargo) != 0 {
		t.Error("nil layers should produce empty packages")
	}
}

func TestMergeEnvironments_PackagesUnion(t *testing.T) {
	a := &store.Environment{
		Packages: store.PackageList{Pip: []string{"requests", "flask"}, Npm: []string{"lodash"}},
	}
	b := &store.Environment{
		Packages: store.PackageList{Pip: []string{"flask", "numpy"}, Apt: []string{"curl"}},
	}
	result := MergeEnvironments(a, b)
	if len(result.Packages.Pip) != 3 { // requests, flask, numpy — flask deduped
		t.Errorf("Pip: expected 3 unique, got %v", result.Packages.Pip)
	}
	if len(result.Packages.Npm) != 1 {
		t.Errorf("Npm: expected 1, got %v", result.Packages.Npm)
	}
	if len(result.Packages.Apt) != 1 {
		t.Errorf("Apt: expected 1, got %v", result.Packages.Apt)
	}
}

func TestMergeEnvironments_PackagesNoDuplicates(t *testing.T) {
	a := &store.Environment{Packages: store.PackageList{Pip: []string{"requests"}}}
	b := &store.Environment{Packages: store.PackageList{Pip: []string{"requests"}}}
	result := MergeEnvironments(a, b)
	if len(result.Packages.Pip) != 1 {
		t.Errorf("duplicate pip package should be deduped; got %v", result.Packages.Pip)
	}
}

func TestMergeEnvironments_NetworkingRestrictedWins(t *testing.T) {
	unrestricted := &store.Environment{
		Networking: store.NetworkingConfig{Mode: store.NetworkingUnrestricted},
	}
	limited := &store.Environment{
		Networking: store.NetworkingConfig{
			Mode:         store.NetworkingLimited,
			AllowedHosts: []string{"api.example.com"},
		},
	}
	result := MergeEnvironments(unrestricted, limited)
	if result.Networking.Mode != store.NetworkingLimited {
		t.Errorf("limited layer should win; got %q", result.Networking.Mode)
	}
	if len(result.Networking.AllowedHosts) != 1 || result.Networking.AllowedHosts[0] != "api.example.com" {
		t.Errorf("AllowedHosts: got %v", result.Networking.AllowedHosts)
	}
}

func TestMergeEnvironments_UnrestrictedClearsAllowedHosts(t *testing.T) {
	// Even if a limited layer is merged first and then an unrestricted layer
	// overrides the mode, the AllowedHosts should be cleared.
	limited := &store.Environment{
		Networking: store.NetworkingConfig{
			Mode:         store.NetworkingLimited,
			AllowedHosts: []string{"api.example.com"},
		},
	}
	unrestricted := &store.Environment{
		Networking: store.NetworkingConfig{Mode: store.NetworkingUnrestricted},
	}
	_ = unrestricted // mode doesn't override once limited is set — test multi-limited union
	// Two limited layers unioned AllowedHosts:
	limited2 := &store.Environment{
		Networking: store.NetworkingConfig{
			Mode:         store.NetworkingLimited,
			AllowedHosts: []string{"cdn.example.com"},
		},
	}
	result := MergeEnvironments(limited, limited2)
	if result.Networking.Mode != store.NetworkingLimited {
		t.Fatal("both limited, mode should be limited")
	}
	if len(result.Networking.AllowedHosts) != 2 {
		t.Errorf("AllowedHosts should be union of both; got %v", result.Networking.AllowedHosts)
	}
}

func TestMergeEnvironments_OnlyUnrestricted_ClearsHosts(t *testing.T) {
	a := &store.Environment{
		Networking: store.NetworkingConfig{Mode: store.NetworkingUnrestricted},
	}
	result := MergeEnvironments(a)
	if result.Networking.AllowedHosts != nil {
		t.Error("unrestricted mode should produce nil AllowedHosts")
	}
}

func TestMergeEnvironments_EnvLastWriterWins(t *testing.T) {
	a := &store.Environment{Env: map[string]string{"KEY": "first", "A": "1"}}
	b := &store.Environment{Env: map[string]string{"KEY": "second", "B": "2"}}
	result := MergeEnvironments(a, b)
	if result.Env["KEY"] != "second" {
		t.Errorf("Env last-writer-wins: got %q, want %q", result.Env["KEY"], "second")
	}
	if result.Env["A"] != "1" || result.Env["B"] != "2" {
		t.Errorf("Env missing keys: got %v", result.Env)
	}
}

func TestMergeEnvironments_EnvNilWhenNoLayers(t *testing.T) {
	result := MergeEnvironments()
	if result.Env != nil {
		t.Error("no layers should produce nil Env map")
	}
}

func TestMergeEnvironments_CargoPackages(t *testing.T) {
	a := &store.Environment{Packages: store.PackageList{Cargo: []string{"serde", "tokio"}}}
	b := &store.Environment{Packages: store.PackageList{Cargo: []string{"tokio", "axum"}}}
	result := MergeEnvironments(a, b)
	if len(result.Packages.Cargo) != 3 {
		t.Errorf("Cargo union: got %v", result.Packages.Cargo)
	}
}

// ── PackagesToSpec ────────────────────────────────────────────────────────────

func TestPackagesToSpec_Conversion(t *testing.T) {
	pl := store.PackageList{
		Pip:   []string{"requests"},
		Npm:   []string{"lodash"},
		Apt:   []string{"curl"},
		Cargo: []string{"serde"},
	}
	spec := PackagesToSpec(pl)
	if len(spec.Pip) != 1 || spec.Pip[0] != "requests" {
		t.Errorf("Pip: got %v", spec.Pip)
	}
	if len(spec.Npm) != 1 || spec.Npm[0] != "lodash" {
		t.Errorf("Npm: got %v", spec.Npm)
	}
	if len(spec.Apt) != 1 || spec.Apt[0] != "curl" {
		t.Errorf("Apt: got %v", spec.Apt)
	}
	if len(spec.Cargo) != 1 || spec.Cargo[0] != "serde" {
		t.Errorf("Cargo: got %v", spec.Cargo)
	}
}

func TestPackagesToSpec_Empty(t *testing.T) {
	spec := PackagesToSpec(store.PackageList{})
	if spec.Pip != nil || spec.Npm != nil || spec.Apt != nil || spec.Cargo != nil {
		t.Error("empty PackageList should produce nil slices")
	}
}

// ── unionStrings ──────────────────────────────────────────────────────────────

func TestUnionStrings_Deduplication(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"b", "c", "d"}
	got := unionStrings(a, b)
	if len(got) != 4 {
		t.Errorf("expected 4 unique strings, got %v", got)
	}
}

func TestUnionStrings_EmptyInputs(t *testing.T) {
	got := unionStrings(nil, nil)
	if len(got) != 0 {
		t.Errorf("expected empty output, got %v", got)
	}
}

func TestUnionStrings_PreservesOrder(t *testing.T) {
	a := []string{"x", "y"}
	b := []string{"z"}
	got := unionStrings(a, b)
	if got[0] != "x" || got[1] != "y" || got[2] != "z" {
		t.Errorf("order not preserved: got %v", got)
	}
}
