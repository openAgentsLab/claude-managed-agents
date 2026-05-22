// Package skill implements the skill system: loading SKILL.md files from the
// filesystem, registering bundled skills, and executing them in either inline
// or fork mode.
package skill

import (
	"path/filepath"
	"strings"
)

// Source identifies where a Skill was loaded from.
type Source string

const (
	SourceBundled Source = "bundled" // compiled into the binary
	SourceDynamic Source = "dynamic" // loaded from the user's DB at session creation
)

// SkillFrontmatter holds the parsed YAML front-matter of a SKILL.md file.
type SkillFrontmatter struct {
	Name                   string         `yaml:"name"`
	Description            string         `yaml:"description"`
	WhenToUse              string         `yaml:"when_to_use"`
	ArgumentHint           string         `yaml:"argument-hint"`
	Model                  string         `yaml:"model"`
	Aliases                []string       `yaml:"aliases"`
	UserInvocable          string         `yaml:"user-invocable"`           // "false" to hide from user
	DisableModelInvocation string         `yaml:"disable-model-invocation"` // "true" to block Skill tool
	Hooks                  map[string]any `yaml:"hooks"`
}

// Skill is a fully loaded skill definition.
type Skill struct {
	Frontmatter SkillFrontmatter
	Content     string // Markdown body after the front-matter block
	Source      Source
	SkillDir    string // absolute path to the directory containing SKILL.md; "" for bundled
}

// Name returns the canonical skill name: front-matter name field if set,
// otherwise the base name of SkillDir.
func (s *Skill) Name() string {
	if s.Frontmatter.Name != "" {
		return s.Frontmatter.Name
	}
	return filepath.Base(s.SkillDir)
}

// IsUserInvocable reports whether users may invoke this skill explicitly.
// Defaults to true; only "false" (case-insensitive) disables it.
func (s *Skill) IsUserInvocable() bool {
	return !strings.EqualFold(s.Frontmatter.UserInvocable, "false")
}

// IsModelInvocable reports whether the LLM may call this skill via the Skill tool.
// Defaults to true; only "true" for DisableModelInvocation disables it.
func (s *Skill) IsModelInvocable() bool {
	return !strings.EqualFold(s.Frontmatter.DisableModelInvocation, "true")
}

// ExpandContent substitutes template variables and returns the final prompt text.
//
//   - $ARGUMENTS            → args (user-supplied argument string)
//   - ${FORGE_SKILL_DIR}    → s.SkillDir
//   - ${FORGE_SESSION_ID}   → sessionID
//
// Trusted ${...} variables are expanded first so that user-controlled args
// cannot inject template tokens that would be re-expanded in a second pass.
func (s *Skill) ExpandContent(args, sessionID string) string {
	r := s.Content
	r = strings.ReplaceAll(r, "${FORGE_SKILL_DIR}", s.SkillDir)
	r = strings.ReplaceAll(r, "${FORGE_SESSION_ID}", sessionID)
	r = strings.ReplaceAll(r, "$ARGUMENTS", args)
	return r
}
