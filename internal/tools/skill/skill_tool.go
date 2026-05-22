// Package skill provides the use_skill tool, an Eino InvokableTool that lets
// the LLM invoke skills registered in a skill.Registry.
package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"forge/internal/skill"
)

type skillToolInput struct {
	Skill string `json:"skill"`
	Args  string `json:"args"`
}

// Tool is an Eino InvokableTool that exposes inline skills to the LLM.
// Each call expands the matched skill's template and returns the result.
type Tool struct {
	registry *skill.Registry
}

// New creates a Tool backed by the given registry.
func New(reg *skill.Registry) *Tool {
	return &Tool{registry: reg}
}

// Info returns the tool schema with a description listing available inline skills.
func (t *Tool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "use_skill",
		Desc: t.buildDescription(),
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"skill": {
				Type:     schema.String,
				Desc:     "Name of the inline skill to invoke",
				Required: true,
			},
			"args": {
				Type:     schema.String,
				Desc:     "Arguments to pass to the skill (optional)",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun expands the requested inline skill and returns the prompt text.
func (t *Tool) InvokableRun(_ context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var input skillToolInput
	if err := json.Unmarshal([]byte(argsJSON), &input); err != nil {
		return "", fmt.Errorf("use_skill: invalid arguments: %w", err)
	}
	if input.Skill == "" {
		return "", fmt.Errorf("use_skill: 'skill' field is required")
	}

	s, ok := t.registry.Find(input.Skill)
	if !ok {
		slog.Debug("use_skill: not found", "skill", input.Skill)
		return "", fmt.Errorf("use_skill: skill %q not found", input.Skill)
	}
	if !s.IsModelInvocable() {
		return "", fmt.Errorf("use_skill: skill %q does not allow model invocation", input.Skill)
	}

	slog.Debug("use_skill: inline", "skill", input.Skill)
	return s.ExpandContent(input.Args, ""), nil
}

func (t *Tool) buildDescription() string {
	var sb strings.Builder
	sb.WriteString("Invoke a named inline skill.\n\n")

	var inlineSkills []*skill.Skill
	for _, s := range t.registry.All() {
		if s.IsModelInvocable() {
			inlineSkills = append(inlineSkills, s)
		}
	}

	if len(inlineSkills) == 0 {
		sb.WriteString("No inline skills are currently available.")
		return sb.String()
	}

	sort.Slice(inlineSkills, func(i, j int) bool { return inlineSkills[i].Name() < inlineSkills[j].Name() })

	sb.WriteString("Available inline skills:\n")
	for _, s := range inlineSkills {
		sb.WriteString("- ")
		sb.WriteString(s.Name())
		if s.Frontmatter.Description != "" {
			sb.WriteString(": ")
			sb.WriteString(s.Frontmatter.Description)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
