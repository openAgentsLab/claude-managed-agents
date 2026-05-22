package permission

import (
	"regexp"
	"strings"
)

// SubCmd is one segment of a pipeline after splitting on |, &&, ||, ;.
type SubCmd struct {
	Raw       string // original segment text
	MainToken string // first command word after stripping env-var assignments
}

// ParsedCommand is the result of fully parsing a raw bash command string.
type ParsedCommand struct {
	Raw        string
	SubCmds    []SubCmd
	Dangerous  bool
	DangerNote string
}

// Parse is the entry point for bash command analysis.
func Parse(raw string) ParsedCommand {
	result := ParsedCommand{Raw: raw}

	if dangerous, note := CheckDangerous(raw); dangerous {
		result.Dangerous = true
		result.DangerNote = note
		return result
	}

	stripped := StripSafeWrappers(raw)
	segments := splitPipeline(stripped)

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		main, _ := extractMainToken(seg)
		if dangerous, note := CheckDangerousToken(main); dangerous {
			result.Dangerous = true
			result.DangerNote = note
			return result
		}
		result.SubCmds = append(result.SubCmds, SubCmd{Raw: seg, MainToken: main})
	}
	return result
}

var safeEnvVars = map[string]bool{
	"GOEXPERIMENT": true, "GOOS": true, "GOARCH": true,
	"CGO_ENABLED": true, "GO111MODULE": true,
	"RUST_BACKTRACE": true, "RUST_LOG": true,
	"NODE_ENV":                       true,
	"PYTHONUNBUFFERED": true, "PYTHONDONTWRITEBYTECODE": true,
	"PYTEST_DISABLE_PLUGIN_AUTOLOAD": true, "PYTEST_DEBUG": true,
	"ANTHROPIC_API_KEY":              true,
	"LANG": true, "LANGUAGE": true, "LC_ALL": true, "LC_CTYPE": true,
	"LC_TIME": true, "CHARSET": true,
	"TERM": true, "COLORTERM": true, "NO_COLOR": true, "FORCE_COLOR": true,
	"TZ":                                        true,
	"LS_COLORS": true, "LSCOLORS": true, "GREP_COLOR": true,
	"GREP_COLORS": true, "GCC_COLORS": true,
	"TIME_STYLE": true, "BLOCK_SIZE": true, "BLOCKSIZE": true,
}

var envVarAssignRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=([A-Za-z0-9_./:-]*)[ \t]+`)

var safeWrapperPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^timeout[ \t]+` +
		`(?:` +
		`(?:--(?:foreground|preserve-status|verbose)` +
		`|--(?:kill-after|signal)=[A-Za-z0-9_.+-]+` +
		`|--(?:kill-after|signal)[ \t]+[A-Za-z0-9_.+-]+` +
		`|-v` +
		`|-[ks][ \t]+[A-Za-z0-9_.+-]+` +
		`|-[ks][A-Za-z0-9_.+-]+` +
		`)[ \t]+)*` +
		`(?:--[ \t]+)?\d+(?:\.\d+)?[smhd]?[ \t]+`),
	regexp.MustCompile(`^time[ \t]+(?:--[ \t]+)?`),
	regexp.MustCompile(`^nice(?:[ \t]+-n[ \t]+-?\d+|[ \t]+-\d+)?[ \t]+(?:--[ \t]+)?`),
	regexp.MustCompile(`^stdbuf(?:[ \t]+-[ioe][LN0-9]+)+[ \t]+(?:--[ \t]+)?`),
	regexp.MustCompile(`^nohup[ \t]+(?:--[ \t]+)?`),
}

// StripSafeWrappers removes safe env-var assignments and wrapper commands
// from the front of cmd.
func StripSafeWrappers(cmd string) string {
	prev := ""
	for cmd != prev {
		prev = cmd
		cmd = stripCommentLines(cmd)
		m := envVarAssignRe.FindStringSubmatch(cmd)
		if m != nil && safeEnvVars[m[1]] {
			cmd = cmd[len(m[0]):]
		}
	}

	prev = ""
	for cmd != prev {
		prev = cmd
		cmd = stripCommentLines(cmd)
		for _, p := range safeWrapperPatterns {
			if loc := p.FindStringIndex(cmd); loc != nil && loc[0] == 0 {
				cmd = cmd[loc[1]:]
				break
			}
		}
	}

	return strings.TrimSpace(cmd)
}

func stripCommentLines(cmd string) string {
	lines := strings.Split(cmd, "\n")
	kept := lines[:0]
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			kept = append(kept, l)
		}
	}
	if len(kept) == 0 {
		return cmd
	}
	return strings.Join(kept, "\n")
}

func splitPipeline(cmd string) []string {
	var segments []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]

		if inSingle {
			current.WriteByte(ch)
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			current.WriteByte(ch)
			if ch == '\\' && i+1 < len(cmd) {
				next := cmd[i+1]
				if next == '$' || next == '`' || next == '"' || next == '\\' || next == '\n' {
					i++
					current.WriteByte(cmd[i])
				}
				continue
			}
			if ch == '"' {
				inDouble = false
			}
			continue
		}

		switch ch {
		case '\'':
			inSingle = true
			current.WriteByte(ch)
		case '"':
			inDouble = true
			current.WriteByte(ch)
		case '\\':
			current.WriteByte(ch)
			if i+1 < len(cmd) {
				i++
				current.WriteByte(cmd[i])
			}
		case '|':
			if i+1 < len(cmd) && cmd[i+1] == '|' {
				i++
			}
			segments = appendSegment(segments, current.String())
			current.Reset()
		case '&':
			if i+1 < len(cmd) && cmd[i+1] == '&' {
				i++
				segments = appendSegment(segments, current.String())
				current.Reset()
			} else {
				segments = appendSegment(segments, current.String())
				current.Reset()
			}
		case ';':
			segments = appendSegment(segments, current.String())
			current.Reset()
		case '\n':
			segments = appendSegment(segments, current.String())
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}
	segments = appendSegment(segments, current.String())
	return segments
}

func appendSegment(segs []string, s string) []string {
	s = strings.TrimSpace(s)
	if s != "" {
		segs = append(segs, s)
	}
	return segs
}

func extractMainToken(seg string) (main, rest string) {
	tokens := strings.Fields(seg)
	if len(tokens) == 0 {
		return "", ""
	}

	i := 0
	for i < len(tokens) {
		t := tokens[i]
		if !isEnvVarAssign(t) {
			break
		}
		varName := t[:strings.Index(t, "=")]
		if !safeEnvVars[varName] {
			break
		}
		i++
	}

	if i >= len(tokens) {
		return "", ""
	}
	main = tokens[i]
	rest = strings.Join(tokens[i+1:], " ")
	return main, rest
}

var envAssignSimpleRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

func isEnvVarAssign(t string) bool {
	return envAssignSimpleRe.MatchString(t)
}
