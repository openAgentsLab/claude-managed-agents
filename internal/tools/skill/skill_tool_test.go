package skill

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"forge/internal/skill"
)

func newTestRegistry() *skill.Registry {
	reg := skill.NewRegistry()
	s := &skill.Skill{
		Frontmatter: skill.SkillFrontmatter{Name: "greet"},
		Content:     "Say hello to $ARGUMENTS!",
		Source:      skill.SourceBundled,
	}
	reg.RegisterBundled(s)
	return reg
}

func newTestRegistryWithDisabled() *skill.Registry {
	reg := skill.NewRegistry()
	reg.RegisterDynamic(&skill.Skill{
		Frontmatter: skill.SkillFrontmatter{
			Name:                   "restricted",
			DisableModelInvocation: "true",
		},
		Content: "secret content",
		Source:  skill.SourceDynamic,
	})
	return reg
}

// ── Info ──────────────────────────────────────────────────────────────────────

func TestSkillTool_Info_Name(t *testing.T) {
	tool := New(newTestRegistry())
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "use_skill" {
		t.Errorf("Name: got %q, want %q", info.Name, "use_skill")
	}
}

func TestSkillTool_Info_ListsSkills(t *testing.T) {
	tool := New(newTestRegistry())
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if !strings.Contains(info.Desc, "greet") {
		t.Errorf("Info.Desc should list registered skills; got: %s", info.Desc)
	}
}

func TestSkillTool_Info_EmptyRegistry(t *testing.T) {
	tool := New(skill.NewRegistry())
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info == nil {
		t.Error("Info should not return nil even for empty registry")
	}
}

// ── InvokableRun ──────────────────────────────────────────────────────────────

func TestSkillTool_InvokableRun_ExpandsContent(t *testing.T) {
	tool := New(newTestRegistry())
	args, _ := json.Marshal(map[string]string{"skill": "greet", "args": "World"})
	out, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(out, "World") {
		t.Errorf("expected 'World' in output; got %q", out)
	}
}

func TestSkillTool_InvokableRun_MissingSkill(t *testing.T) {
	tool := New(skill.NewRegistry())
	args, _ := json.Marshal(map[string]string{"skill": "nonexistent"})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Error("expected error for unknown skill, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention skill name; got: %v", err)
	}
}

func TestSkillTool_InvokableRun_EmptySkillField(t *testing.T) {
	tool := New(newTestRegistry())
	args, _ := json.Marshal(map[string]string{"skill": ""})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Error("expected error for empty skill name, got nil")
	}
}

func TestSkillTool_InvokableRun_DisabledModelInvocation(t *testing.T) {
	tool := New(newTestRegistryWithDisabled())
	args, _ := json.Marshal(map[string]string{"skill": "restricted"})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Error("expected error for skill with model invocation disabled, got nil")
	}
}

func TestSkillTool_InvokableRun_InvalidJSON(t *testing.T) {
	tool := New(newTestRegistry())
	_, err := tool.InvokableRun(context.Background(), "not-json")
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestSkillTool_InvokableRun_NoArgs(t *testing.T) {
	tool := New(newTestRegistry())
	// args field is optional — empty is valid
	args, _ := json.Marshal(map[string]string{"skill": "greet"})
	out, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty output even with no args")
	}
}
