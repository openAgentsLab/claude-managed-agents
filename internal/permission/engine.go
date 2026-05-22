package permission

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// writeFileTools is the set of tool names that write to the filesystem.
var writeFileTools = map[string]bool{
	"write_file":  true,
	"file_edit":   true,
	"delete_file": true,
	"move_file":   true,
	"delete_dir":  true,
}

// Decision is the result of a permission check.
type Decision struct {
	Behavior    Behavior
	Message     string
	ReasonType  string // "rule" | "mode" | "safetyCheck" | "other"
	MatchedRule *Rule
}

// Engine holds the active permission rules and mode.
// All methods are safe for concurrent use.
type Engine struct {
	mu    sync.RWMutex
	mode  Mode
	rules []Rule // maintained in Source ascending order (lower priority first)
}

// NewEngine creates an Engine with the given mode and no rules.
func NewEngine(mode Mode) *Engine {
	return &Engine{mode: mode}
}

// SetMode changes the permission mode at runtime.
func (e *Engine) SetMode(m Mode) {
	e.mu.Lock()
	e.mode = m
	e.mu.Unlock()
}

// WithMode returns a copy of the Engine with mode replaced. Rules are
// preserved; the copy is independent of the original.
func (e *Engine) WithMode(m Mode) *Engine {
	e.mu.RLock()
	rules := make([]Rule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()
	return &Engine{mode: m, rules: rules}
}

// Mode returns the current permission mode.
func (e *Engine) Mode() Mode {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.mode
}

// AddRule inserts a rule, maintaining ascending Source order so that
// higher-priority sources (larger Source value) sit at the end of the slice.
// check* functions iterate from the end, so the last matching rule wins.
func (e *Engine) AddRule(r Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Find insertion point: last position where Source ≤ r.Source.
	i := len(e.rules)
	for i > 0 && e.rules[i-1].Source > r.Source {
		i--
	}
	e.rules = append(e.rules, Rule{})
	for j := len(e.rules) - 1; j > i; j-- {
		e.rules[j] = e.rules[j-1]
	}
	e.rules[i] = r
}

// Check evaluates a tool invocation and returns the permission decision.
func (e *Engine) Check(toolName, argsJSON string, readOnly bool) Decision {
	e.mu.RLock()
	mode := e.mode
	rules := make([]Rule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	// Run the full rule + safety check regardless of mode, so that explicit deny
	// rules and dangerous-command detection are never bypassed by plan mode.
	var d Decision
	if toolName == "bash" {
		d = checkBash(argsJSON, rules)
	} else {
		d = checkGeneric(toolName, argsJSON, rules)
	}

	if mode == ModePlan {
		if d.Behavior == BehaviorDeny {
			return d
		}
		if readOnly {
			return Decision{Behavior: BehaviorAllow, ReasonType: "mode",
				Message: "plan mode: read-only tool allowed"}
		}
		return Decision{Behavior: BehaviorDeny, ReasonType: "mode",
			Message: "plan mode: write operations are not permitted"}
	}

	return d
}

// ---------------------------------------------------------------------------
// Bash check
// ---------------------------------------------------------------------------

type bashArgs struct {
	Command string `json:"command"`
}

func checkBash(argsJSON string, rules []Rule) Decision {
	var args bashArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Command == "" {
		return Decision{Behavior: BehaviorAllow, ReasonType: "other",
			Message: "could not parse bash command arguments"}
	}

	parsed := Parse(args.Command)
	if parsed.Dangerous {
		return Decision{
			Behavior:   BehaviorDeny,
			ReasonType: "safetyCheck",
			Message:    "dangerous shell construct detected: " + parsed.DangerNote,
		}
	}

	if len(parsed.SubCmds) == 0 {
		return Decision{Behavior: BehaviorAllow, ReasonType: "other", Message: "empty command"}
	}

	for _, sub := range parsed.SubCmds {
		d := checkBashSubCmd(sub, rules)
		if d.Behavior == BehaviorDeny {
			return d
		}
	}
	return Decision{Behavior: BehaviorAllow, ReasonType: "rule"}
}

func checkBashSubCmd(sub SubCmd, rules []Rule) Decision {
	cmd := sub.Raw

	for i := len(rules) - 1; i >= 0; i-- {
		r := rules[i]
		if r.Behavior != BehaviorDeny || !ruleAppliesToTool(r, "bash") {
			continue
		}
		if matchBashPattern(r.Pattern, cmd) {
			rv := r
			return Decision{
				Behavior:    BehaviorDeny,
				ReasonType:  "rule",
				MatchedRule: &rv,
				Message:     "denied by rule: " + r.String(),
			}
		}
	}

	for i := len(rules) - 1; i >= 0; i-- {
		r := rules[i]
		if r.Behavior != BehaviorAsk || !ruleAppliesToTool(r, "bash") {
			continue
		}
		if matchBashPattern(r.Pattern, cmd) {
			rv := r
			return Decision{Behavior: BehaviorAsk, ReasonType: "rule", MatchedRule: &rv}
		}
	}

	for i := len(rules) - 1; i >= 0; i-- {
		r := rules[i]
		if r.Behavior != BehaviorAllow || !ruleAppliesToTool(r, "bash") {
			continue
		}
		if matchBashPattern(r.Pattern, cmd) {
			return Decision{Behavior: BehaviorAllow, ReasonType: "rule"}
		}
	}

	return Decision{Behavior: BehaviorAllow, ReasonType: "rule"}
}

// ---------------------------------------------------------------------------
// Generic (non-Bash) check
// ---------------------------------------------------------------------------

type filePathArgs struct {
	FilePath string `json:"file_path"`
	Src      string `json:"src"`
	Dst      string `json:"dst"`
}

func checkGeneric(toolName, argsJSON string, rules []Rule) Decision {
	if writeFileTools[toolName] {
		var fp filePathArgs
		if err := json.Unmarshal([]byte(argsJSON), &fp); err == nil {
			for _, t := range []string{fp.FilePath, fp.Src, fp.Dst} {
				if t != "" && IsProtectedPath(t) {
					return Decision{
						Behavior:   BehaviorDeny,
						ReasonType: "safetyCheck",
						Message:    ProtectedPathMessage(t),
					}
				}
			}
		}
	}

	for i := len(rules) - 1; i >= 0; i-- {
		r := rules[i]
		if r.Behavior != BehaviorDeny || !ruleAppliesToTool(r, toolName) {
			continue
		}
		if matchGenericPattern(r.Pattern, argsJSON) {
			rv := r
			return Decision{
				Behavior:    BehaviorDeny,
				ReasonType:  "rule",
				MatchedRule: &rv,
				Message:     "denied by rule: " + r.String(),
			}
		}
	}

	for i := len(rules) - 1; i >= 0; i-- {
		r := rules[i]
		if r.Behavior != BehaviorAsk || !ruleAppliesToTool(r, toolName) {
			continue
		}
		if matchGenericPattern(r.Pattern, argsJSON) {
			rv := r
			return Decision{Behavior: BehaviorAsk, ReasonType: "rule", MatchedRule: &rv}
		}
	}

	for i := len(rules) - 1; i >= 0; i-- {
		r := rules[i]
		if r.Behavior != BehaviorAllow || !ruleAppliesToTool(r, toolName) {
			continue
		}
		if matchGenericPattern(r.Pattern, argsJSON) {
			return Decision{Behavior: BehaviorAllow, ReasonType: "rule"}
		}
	}

	return Decision{Behavior: BehaviorAllow, ReasonType: "rule"}
}


// ---------------------------------------------------------------------------
// Rule matching helpers
// ---------------------------------------------------------------------------

func ruleAppliesToTool(r Rule, toolName string) bool {
	if r.ToolName == "" {
		return true
	}
	return strings.EqualFold(r.ToolName, toolName)
}

func matchBashPattern(pattern, cmd string) bool {
	if pattern == "" {
		return true
	}
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, ":*")
		cmdLower := strings.ToLower(cmd)
		prefixLower := strings.ToLower(prefix)
		return cmdLower == prefixLower || strings.HasPrefix(cmdLower, prefixLower+" ")
	}
	if hasUnescapedWildcard(pattern) {
		return matchWildcard(pattern, cmd)
	}
	return strings.EqualFold(cmd, pattern)
}

type genericURLArgs struct {
	URL string `json:"url"`
}

type genericPathArgs struct {
	FilePath string `json:"file_path"`
	Path     string `json:"path"`
}

func matchGenericPattern(pattern, argsJSON string) bool {
	if pattern == "" {
		return true
	}
	if after, ok := strings.CutPrefix(pattern, "domain:"); ok {
		return matchDomainPattern(after, argsJSON)
	}
	if after, ok := strings.CutPrefix(pattern, "path:"); ok {
		return matchPathPattern(after, argsJSON)
	}
	return strings.Contains(strings.ToLower(argsJSON), strings.ToLower(pattern))
}

func matchDomainPattern(wantDomain, argsJSON string) bool {
	var a genericURLArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || a.URL == "" {
		return strings.Contains(strings.ToLower(argsJSON), strings.ToLower(wantDomain))
	}
	host := extractHost(a.URL)
	want := strings.ToLower(wantDomain)
	hostLow := strings.ToLower(host)
	return hostLow == want || strings.HasSuffix(hostLow, "."+want)
}

func extractHost(rawURL string) string {
	s := rawURL
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	} else if strings.HasPrefix(s, "//") {
		s = s[2:]
	}
	if strings.HasPrefix(s, "[") {
		if end := strings.Index(s, "]"); end >= 0 {
			return s[1:end]
		}
		return s
	}
	for _, sep := range []string{"/", "?", "#"} {
		if i := strings.Index(s, sep); i >= 0 {
			s = s[:i]
		}
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[:i]
	}
	return s
}

func matchPathPattern(wantPrefix, argsJSON string) bool {
	var a genericPathArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return false
	}
	target := a.FilePath
	if target == "" {
		target = a.Path
	}
	if target == "" {
		return false
	}
	// filepath.Clean resolves ".." segments before prefix matching so that
	// paths like "/protected/../etc/passwd" cannot bypass deny rules.
	return strings.HasPrefix(
		strings.ToLower(filepath.ToSlash(filepath.Clean(target))),
		strings.ToLower(filepath.ToSlash(wantPrefix)),
	)
}

func hasUnescapedWildcard(pattern string) bool {
	if strings.HasSuffix(pattern, ":*") {
		return false
	}
	for i, ch := range pattern {
		if ch != '*' {
			continue
		}
		backslashes := 0
		for j := i - 1; j >= 0 && pattern[j] == '\\'; j-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return true
		}
	}
	return false
}

func matchWildcard(pattern, cmd string) bool {
	re := buildWildcardRegexp(pattern)
	return re.MatchString(cmd)
}

const (
	escapedStarPlaceholder      = "\x00STAR\x00"
	escapedBackslashPlaceholder = "\x00BSLASH\x00"
)

func buildWildcardRegexp(pattern string) *regexp.Regexp {
	var processed strings.Builder
	i := 0
	for i < len(pattern) {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			switch pattern[i+1] {
			case '*':
				processed.WriteString(escapedStarPlaceholder)
				i += 2
				continue
			case '\\':
				processed.WriteString(escapedBackslashPlaceholder)
				i += 2
				continue
			}
		}
		processed.WriteByte(pattern[i])
		i++
	}

	escaped := regexp.QuoteMeta(processed.String())
	withWild := strings.ReplaceAll(escaped, `\*`, `.*`)
	withWild = strings.ReplaceAll(withWild, escapedStarPlaceholder, `\*`)
	withWild = strings.ReplaceAll(withWild, escapedBackslashPlaceholder, `\\`)

	unescapedStars := strings.Count(processed.String(), "*")
	if strings.HasSuffix(withWild, ` .*`) && unescapedStars == 1 {
		withWild = withWild[:len(withWild)-3] + `( .*)?`
	}

	re, err := regexp.Compile(`(?si)^` + withWild + `$`)
	if err != nil {
		return regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(pattern) + `$`)
	}
	return re
}
