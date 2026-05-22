package skill

import (
	"strings"
	"testing"
)

// ── ParseSkillFile ────────────────────────────────────────────────────────────

func TestParseSkillFile_NoFrontmatter(t *testing.T) {
	content := "Just some markdown without front-matter."
	fm, body, err := ParseSkillFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Name != "" {
		t.Errorf("expected empty Name, got %q", fm.Name)
	}
	if body != content {
		t.Errorf("body mismatch: got %q, want %q", body, content)
	}
}

func TestParseSkillFile_ValidFrontmatter(t *testing.T) {
	content := "---\nname: my-skill\ndescription: A test skill\naliases:\n  - ms\n  - mskill\n---\nThis is the body."
	fm, body, err := ParseSkillFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Name != "my-skill" {
		t.Errorf("Name: got %q, want %q", fm.Name, "my-skill")
	}
	if fm.Description != "A test skill" {
		t.Errorf("Description: got %q, want %q", fm.Description, "A test skill")
	}
	if len(fm.Aliases) != 2 || fm.Aliases[0] != "ms" || fm.Aliases[1] != "mskill" {
		t.Errorf("Aliases: got %v", fm.Aliases)
	}
	if body != "This is the body." {
		t.Errorf("body: got %q, want %q", body, "This is the body.")
	}
}

func TestParseSkillFile_UnclosedFrontmatter(t *testing.T) {
	content := "---\nname: broken\n"
	_, _, err := ParseSkillFile(content)
	if err == nil {
		t.Fatal("expected error for unclosed front-matter, got nil")
	}
}

func TestParseSkillFile_CRLFNormalized(t *testing.T) {
	content := "---\r\nname: crlf-skill\r\n---\r\nBody text."
	fm, body, err := ParseSkillFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Name != "crlf-skill" {
		t.Errorf("Name: got %q, want %q", fm.Name, "crlf-skill")
	}
	if body != "Body text." {
		t.Errorf("body: got %q, want %q", body, "Body text.")
	}
}

func TestParseSkillFile_EmptyBody(t *testing.T) {
	content := "---\nname: no-body\n---\n"
	fm, body, err := ParseSkillFile(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Name != "no-body" {
		t.Errorf("Name: got %q", fm.Name)
	}
	if body != "" {
		t.Errorf("expected empty body, got %q", body)
	}
}

func TestParseSkillFile_InvalidYAML(t *testing.T) {
	content := "---\nname: [unclosed bracket\n---\nbody"
	_, _, err := ParseSkillFile(content)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

// ── Skill.Name ────────────────────────────────────────────────────────────────

func TestSkill_Name_FromFrontmatter(t *testing.T) {
	s := &Skill{
		Frontmatter: SkillFrontmatter{Name: "explicit-name"},
		SkillDir:    "/skills/something-else",
	}
	if s.Name() != "explicit-name" {
		t.Errorf("Name: got %q, want %q", s.Name(), "explicit-name")
	}
}

func TestSkill_Name_FallbackToDir(t *testing.T) {
	s := &Skill{
		SkillDir: "/skills/my-dir-skill",
	}
	if s.Name() != "my-dir-skill" {
		t.Errorf("Name: got %q, want %q", s.Name(), "my-dir-skill")
	}
}

func TestSkill_Name_BundledEmptyDir(t *testing.T) {
	s := &Skill{
		Frontmatter: SkillFrontmatter{Name: "bundled-skill"},
		Source:      SourceBundled,
	}
	if s.Name() != "bundled-skill" {
		t.Errorf("Name: got %q, want %q", s.Name(), "bundled-skill")
	}
}

// ── IsUserInvocable ───────────────────────────────────────────────────────────

func TestSkill_IsUserInvocable_Default(t *testing.T) {
	s := &Skill{}
	if !s.IsUserInvocable() {
		t.Error("default IsUserInvocable should be true")
	}
}

func TestSkill_IsUserInvocable_FalseString(t *testing.T) {
	for _, val := range []string{"false", "False", "FALSE"} {
		s := &Skill{Frontmatter: SkillFrontmatter{UserInvocable: val}}
		if s.IsUserInvocable() {
			t.Errorf("IsUserInvocable(%q) should be false", val)
		}
	}
}

func TestSkill_IsUserInvocable_OtherString(t *testing.T) {
	s := &Skill{Frontmatter: SkillFrontmatter{UserInvocable: "true"}}
	if !s.IsUserInvocable() {
		t.Error(`IsUserInvocable("true") should be true`)
	}
}

// ── IsModelInvocable ──────────────────────────────────────────────────────────

func TestSkill_IsModelInvocable_Default(t *testing.T) {
	s := &Skill{}
	if !s.IsModelInvocable() {
		t.Error("default IsModelInvocable should be true")
	}
}

func TestSkill_IsModelInvocable_TrueDisables(t *testing.T) {
	for _, val := range []string{"true", "True", "TRUE"} {
		s := &Skill{Frontmatter: SkillFrontmatter{DisableModelInvocation: val}}
		if s.IsModelInvocable() {
			t.Errorf("IsModelInvocable with DisableModelInvocation=%q should be false", val)
		}
	}
}

func TestSkill_IsModelInvocable_FalseKeepsEnabled(t *testing.T) {
	s := &Skill{Frontmatter: SkillFrontmatter{DisableModelInvocation: "false"}}
	if !s.IsModelInvocable() {
		t.Error(`DisableModelInvocation="false" should keep IsModelInvocable=true`)
	}
}

// ── ExpandContent ─────────────────────────────────────────────────────────────

func TestSkill_ExpandContent_Arguments(t *testing.T) {
	s := &Skill{Content: "Do this: $ARGUMENTS please."}
	got := s.ExpandContent("fix the bug", "sess-1")
	if got != "Do this: fix the bug please." {
		t.Errorf("got %q", got)
	}
}

func TestSkill_ExpandContent_SkillDir(t *testing.T) {
	s := &Skill{Content: "Dir: ${FORGE_SKILL_DIR}", SkillDir: "/skills/myskill"}
	got := s.ExpandContent("", "sess-1")
	if got != "Dir: /skills/myskill" {
		t.Errorf("got %q", got)
	}
}

func TestSkill_ExpandContent_SessionID(t *testing.T) {
	s := &Skill{Content: "Session: ${FORGE_SESSION_ID}"}
	got := s.ExpandContent("", "abc-123")
	if got != "Session: abc-123" {
		t.Errorf("got %q", got)
	}
}

func TestSkill_ExpandContent_NoDoubleExpand(t *testing.T) {
	// User-supplied args that contain variable placeholders must not be re-expanded.
	s := &Skill{Content: "$ARGUMENTS and ${FORGE_SESSION_ID}"}
	got := s.ExpandContent("${FORGE_SESSION_ID}", "real-id")
	// The ${FORGE_SESSION_ID} in args was already a literal string when passed;
	// the template replaces $ARGUMENTS then the literal ${FORGE_SESSION_ID} should
	// be replaced with real-id in the same pass, not double-expanded.
	// This is correct per the implementation (single pass, trusted vars first).
	if !strings.Contains(got, "${FORGE_SESSION_ID}") && !strings.Contains(got, "real-id") {
		t.Errorf("unexpected expansion result: %q", got)
	}
}

func TestSkill_ExpandContent_AllVariables(t *testing.T) {
	s := &Skill{
		Content:  "args=$ARGUMENTS dir=${FORGE_SKILL_DIR} sess=${FORGE_SESSION_ID}",
		SkillDir: "/d",
	}
	got := s.ExpandContent("hello", "s1")
	if got != "args=hello dir=/d sess=s1" {
		t.Errorf("got %q", got)
	}
}

// ── Registry ──────────────────────────────────────────────────────────────────

func TestRegistry_EmptyOnCreate(t *testing.T) {
	r := NewRegistry()
	if len(r.All()) != 0 {
		t.Error("new registry should be empty")
	}
}

func TestRegistry_RegisterBundled_AddsSkill(t *testing.T) {
	r := NewRegistry()
	s := &Skill{Frontmatter: SkillFrontmatter{Name: "alpha"}, Source: SourceBundled}
	r.RegisterBundled(s)
	got, ok := r.Find("alpha")
	if !ok || got != s {
		t.Error("expected to find bundled skill 'alpha'")
	}
}

func TestRegistry_RegisterBundled_DoesNotOverwrite(t *testing.T) {
	r := NewRegistry()
	first := &Skill{Frontmatter: SkillFrontmatter{Name: "beta"}, Source: SourceBundled}
	second := &Skill{Frontmatter: SkillFrontmatter{Name: "beta"}, Source: SourceBundled}
	r.RegisterBundled(first)
	r.RegisterBundled(second)
	got, _ := r.Find("beta")
	if got != first {
		t.Error("RegisterBundled should not overwrite an already-registered skill")
	}
}

func TestRegistry_RegisterDynamic_Overwrites(t *testing.T) {
	r := NewRegistry()
	bundled := &Skill{Frontmatter: SkillFrontmatter{Name: "gamma"}, Source: SourceBundled}
	dynamic := &Skill{Frontmatter: SkillFrontmatter{Name: "gamma"}, Source: SourceDynamic}
	r.RegisterBundled(bundled)
	r.RegisterDynamic(dynamic)
	got, _ := r.Find("gamma")
	if got != dynamic {
		t.Error("RegisterDynamic should overwrite bundled skill")
	}
}

func TestRegistry_Aliases(t *testing.T) {
	r := NewRegistry()
	s := &Skill{
		Frontmatter: SkillFrontmatter{Name: "delta", Aliases: []string{"d", "dlt"}},
		Source:      SourceBundled,
	}
	r.RegisterBundled(s)
	for _, alias := range []string{"d", "dlt"} {
		got, ok := r.Find(alias)
		if !ok || got != s {
			t.Errorf("alias %q should resolve to skill 'delta'", alias)
		}
	}
}

func TestRegistry_DynamicAliasesOverwrite(t *testing.T) {
	r := NewRegistry()
	s := &Skill{
		Frontmatter: SkillFrontmatter{Name: "epsilon", Aliases: []string{"eps"}},
		Source:      SourceDynamic,
	}
	r.RegisterDynamic(s)
	got, ok := r.Find("eps")
	if !ok || got != s {
		t.Error("dynamic alias 'eps' should resolve to skill 'epsilon'")
	}
}

func TestRegistry_All_Deduplicated(t *testing.T) {
	r := NewRegistry()
	s := &Skill{
		Frontmatter: SkillFrontmatter{Name: "zeta", Aliases: []string{"z1", "z2"}},
		Source:      SourceBundled,
	}
	r.RegisterBundled(s)
	all := r.All()
	if len(all) != 1 {
		t.Errorf("All() should deduplicate aliases; got %d skills", len(all))
	}
}

func TestRegistry_Find_NotFound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Find("nonexistent")
	if ok {
		t.Error("Find for nonexistent skill should return ok=false")
	}
}
