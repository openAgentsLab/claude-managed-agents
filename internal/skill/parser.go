package skill

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseSkillFile parses the content of a SKILL.md file and returns the parsed
// front-matter and the Markdown body that follows it.
//
// If the file has no front-matter (does not start with "---"), an empty
// SkillFrontmatter is returned and the whole content is treated as the body.
func ParseSkillFile(content string) (SkillFrontmatter, string, error) {
	if !strings.HasPrefix(content, "---") {
		return SkillFrontmatter{}, content, nil
	}

	// Normalise line endings so the parser is CRLF-agnostic.
	rest := strings.ReplaceAll(content[3:], "\r\n", "\n")

	endIdx := strings.Index(rest, "\n---")
	if endIdx == -1 {
		return SkillFrontmatter{}, "", fmt.Errorf("skill: unclosed front-matter block")
	}

	yamlBlock := rest[:endIdx]
	bodyStart := endIdx + 4 // len("\n---") == 4
	if bodyStart < len(rest) && rest[bodyStart] == '\n' {
		bodyStart++
	}
	body := strings.TrimSpace(rest[bodyStart:])

	var fm SkillFrontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return SkillFrontmatter{}, "", fmt.Errorf("skill: parse front-matter: %w", err)
	}

	return fm, body, nil
}
