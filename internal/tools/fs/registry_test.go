package fs

import (
	"context"
	"testing"
)

func toolNames(t *testing.T) []string {
	t.Helper()
	reg, err := NewFsRegistry(context.Background())
	if err != nil {
		t.Fatalf("NewFsRegistry: %v", err)
	}
	ctx := context.Background()
	var names []string
	for _, bt := range reg.Tools() {
		info, err := bt.Info(ctx)
		if err != nil || info == nil {
			continue
		}
		names = append(names, info.Name)
	}
	return names
}

func TestNewFsRegistry_HasToolsAfterCreate(t *testing.T) {
	if len(toolNames(t)) == 0 {
		t.Error("expected at least one tool, got 0")
	}
}

func TestNewFsRegistry_ContainsFilesystemTools(t *testing.T) {
	names := make(map[string]bool)
	for _, n := range toolNames(t) {
		names[n] = true
	}
	for _, want := range []string{"read_file", "write_file", "list_dir", "glob"} {
		if !names[want] {
			t.Errorf("expected %q in FsRegistry", want)
		}
	}
}

// bash is an exec tool registered via ExecRegistry — it must NOT appear here.
func TestNewFsRegistry_DoesNotContainBashTool(t *testing.T) {
	for _, name := range toolNames(t) {
		if name == "bash" {
			t.Error("bash must not be in FsRegistry; it belongs to ExecRegistry (sandboxed)")
		}
	}
}

func TestNewFsRegistry_ToolsConsistent(t *testing.T) {
	reg, err := NewFsRegistry(context.Background())
	if err != nil {
		t.Fatalf("NewFsRegistry: %v", err)
	}
	if len(reg.Tools()) != len(reg.Tools()) {
		t.Error("Tools() should return consistent results")
	}
}
