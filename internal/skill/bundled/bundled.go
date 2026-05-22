// Package bundled registers the built-in skills compiled into the binary via
// Go's embed directive.
//
// Call Init(registry) once at program startup (before loading filesystem
// skills) so that user/project skills can override bundled ones.
package bundled

import (
	"embed"
	"io/fs"
	"log/slog"
	"path/filepath"

	"forge/internal/skill"
)

//go:embed skills
var skillsFS embed.FS

// Init registers all bundled SKILL.md files into registry.
// Each sub-directory of the embedded "skills/" tree is treated as a named
// skill; directories without a SKILL.md are silently skipped.
func Init(registry *skill.Registry) {
	entries, err := skillsFS.ReadDir("skills")
	if err != nil {
		slog.Debug("skill/bundled: no embedded skills directory", "error", err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		mdPath := filepath.Join("skills", entry.Name(), "SKILL.md")
		data, err := fs.ReadFile(skillsFS, mdPath)
		if err != nil {
			continue
		}
		fm, body, err := skill.ParseSkillFile(string(data))
		if err != nil {
			slog.Debug("skill/bundled: failed to parse SKILL.md", "dir", entry.Name(), "error", err)
			continue
		}
		s := &skill.Skill{
			Frontmatter: fm,
			Content:     body,
			Source:      skill.SourceBundled,
			SkillDir:    "",
		}
		if s.Frontmatter.Name == "" {
			s.Frontmatter.Name = entry.Name()
		}
		slog.Debug("skill/bundled: registering", "name", s.Name())
		registry.RegisterBundled(s)
	}
}
