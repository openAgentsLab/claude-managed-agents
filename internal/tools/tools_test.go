package tools

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ── mockBaseTool ──────────────────────────────────────────────────────────────

type mockBaseTool struct{ name string }

func (m *mockBaseTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: m.name}, nil
}

// ── Static ────────────────────────────────────────────────────────────────────

func TestStatic_ReturnsTools(t *testing.T) {
	tools := []tool.BaseTool{
		&mockBaseTool{name: "a"},
		&mockBaseTool{name: "b"},
	}
	reg := Static(tools)
	got := reg.Tools()
	if len(got) != 2 {
		t.Errorf("expected 2 tools, got %d", len(got))
	}
}

func TestStatic_NilInput(t *testing.T) {
	reg := Static(nil)
	got := reg.Tools()
	if got != nil {
		t.Errorf("Static(nil).Tools() should return nil, got %v", got)
	}
}

func TestStatic_Empty(t *testing.T) {
	reg := Static([]tool.BaseTool{})
	got := reg.Tools()
	if len(got) != 0 {
		t.Errorf("expected empty tools, got %d", len(got))
	}
}

// ── Merge ─────────────────────────────────────────────────────────────────────

func TestMerge_CombinesTools(t *testing.T) {
	a := Static([]tool.BaseTool{&mockBaseTool{name: "tool-a"}})
	b := Static([]tool.BaseTool{&mockBaseTool{name: "tool-b"}})
	merged := Merge(a, b)
	got := merged.Tools()
	if len(got) != 2 {
		t.Errorf("expected 2 tools from merge, got %d", len(got))
	}
}

func TestMerge_NoRegistries(t *testing.T) {
	merged := Merge()
	got := merged.Tools()
	if len(got) != 0 {
		t.Errorf("merge of nothing should return empty tools, got %d", len(got))
	}
}

func TestMerge_SingleRegistry(t *testing.T) {
	a := Static([]tool.BaseTool{&mockBaseTool{name: "only"}})
	merged := Merge(a)
	got := merged.Tools()
	if len(got) != 1 {
		t.Errorf("expected 1 tool, got %d", len(got))
	}
}

func TestMerge_OrderPreserved(t *testing.T) {
	a := Static([]tool.BaseTool{&mockBaseTool{name: "first"}})
	b := Static([]tool.BaseTool{&mockBaseTool{name: "second"}})
	c := Static([]tool.BaseTool{&mockBaseTool{name: "third"}})
	merged := Merge(a, b, c)
	got := merged.Tools()
	if len(got) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(got))
	}
	names := make([]string, 3)
	for i, t := range got {
		info, _ := t.Info(context.Background())
		names[i] = info.Name
	}
	if names[0] != "first" || names[1] != "second" || names[2] != "third" {
		t.Errorf("tool order not preserved: got %v", names)
	}
}

func TestMerge_ThreeRegistriesWithOverlap(t *testing.T) {
	// Merge does NOT deduplicate — it's a simple concat.
	a := Static([]tool.BaseTool{&mockBaseTool{name: "dup"}})
	b := Static([]tool.BaseTool{&mockBaseTool{name: "dup"}})
	merged := Merge(a, b)
	got := merged.Tools()
	if len(got) != 2 {
		t.Errorf("Merge does not dedup — expected 2 tools for 2 registries, got %d", len(got))
	}
}
