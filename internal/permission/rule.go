package permission

import (
	"fmt"
	"strings"
)

// RuleSource identifies where a permission rule came from.
// Higher numeric values have higher priority; when two rules conflict
// the one with the higher source wins.
type RuleSource int

const (
	SourceUser    RuleSource = iota // global platform defaults — lowest priority
	SourceProject                   // tenant-level rules
	SourceLocal                     // local overrides (not committed)
	SourceCLIArg                    // startup config flags — highest priority
)

// Behavior is the outcome of a permission check.
type Behavior string

const (
	BehaviorAllow Behavior = "allow"
	BehaviorDeny  Behavior = "deny"
	BehaviorAsk   Behavior = "ask" // human-in-the-loop: pause and require user confirmation
)

// Rule is a single permission entry.
//
//   - ToolName is the canonical tool name (e.g. "bash", "read_file").
//     An empty ToolName matches every tool.
//   - Pattern is an optional command/argument filter whose syntax depends on
//     the tool:
//   - For Bash: "git:*" (prefix), "npm run *" (wildcard), or an exact command.
//   - For other tools: matched case-insensitively against the raw argsJSON.
//     An empty Pattern matches every invocation of the tool.
//   - Behavior is allow, deny, or ask.
//   - Source determines priority when rules conflict.
type Rule struct {
	ToolName string
	Pattern  string
	Behavior Behavior
	Source   RuleSource
}

// String returns the canonical string representation used in settings files,
// e.g. "Bash(git:*)", "read_file", "WebFetch(domain:github.com)".
func (r Rule) String() string {
	name := r.ToolName
	if name == "" {
		name = "*"
	}
	if r.Pattern == "" {
		return name
	}
	return fmt.Sprintf("%s(%s)", name, r.Pattern)
}

// ParseRuleString parses a rule string of the form "ToolName" or
// "ToolName(Pattern)" into a Rule with the given behavior and source.
func ParseRuleString(s string, behavior Behavior, source RuleSource) (Rule, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Rule{}, fmt.Errorf("empty rule string")
	}
	open := strings.Index(s, "(")
	if open == -1 {
		return Rule{ToolName: s, Behavior: behavior, Source: source}, nil
	}
	if !strings.HasSuffix(s, ")") {
		return Rule{}, fmt.Errorf("rule %q: missing closing parenthesis", s)
	}
	toolName := strings.TrimSpace(s[:open])
	pattern := strings.TrimSpace(s[open+1 : len(s)-1])
	if toolName == "" {
		return Rule{}, fmt.Errorf("rule %q: empty tool name", s)
	}
	return Rule{ToolName: toolName, Pattern: pattern, Behavior: behavior, Source: source}, nil
}
