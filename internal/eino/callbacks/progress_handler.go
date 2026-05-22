package callbacks

import (
	"encoding/json"
	"fmt"
	"strings"
)

// formatArgs produces a short human-readable summary of a tool's JSON args.
// It tries to surface the most meaningful field for each known tool; for
// unknown tools it falls back to a truncated raw-JSON representation.
func formatArgs(toolName, argsJSON string) string {
	if argsJSON == "" {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return truncate(argsJSON, 120)
	}

	switch toolName {
	// ── exec ──────────────────────────────────────────────────────────────────
	case "bash":
		return truncate(strField(m, "command"), 120)

	// ── filesystem ────────────────────────────────────────────────────────────
	case "read_file", "write_file":
		s := strField(m, "file_path")
		if off := m["offset"]; off != nil {
			s += fmt.Sprintf("  (offset=%v", off)
			if lim := m["limit"]; lim != nil {
				s += fmt.Sprintf(" limit=%v", lim)
			}
			s += ")"
		}
		return s
	case "file_edit":
		path := strField(m, "file_path")
		old := strField(m, "old_string")
		if len(old) > 40 {
			old = old[:40] + "…"
		}
		old = strings.ReplaceAll(old, "\n", "↵")
		return fmt.Sprintf("%s  old=%q", path, old)
	case "glob":
		s := strField(m, "pattern")
		if p := strField(m, "path"); p != "" {
			s += "  in " + p
		}
		return s
	case "grep":
		s := strField(m, "pattern")
		if g := strField(m, "glob"); g != "" {
			s += "  " + g
		}
		if p := strField(m, "path"); p != "" {
			s += "  in " + p
		}
		return s
	case "list_dir":
		return strField(m, "dir_path")
	case "delete_file":
		return strField(m, "file_path")
	case "delete_dir":
		return strField(m, "dir_path")
	case "move_file":
		return strField(m, "src_path") + " → " + strField(m, "dst_path")

	// ── git ───────────────────────────────────────────────────────────────────
	case "git_status":
		return ""
	case "git_add":
		if paths := stringsField(m, "paths"); len(paths) > 0 {
			return truncate(strings.Join(paths, " "), 120)
		}
		return ""
	case "git_commit":
		return truncate(strField(m, "message"), 80)
	case "git_diff":
		var parts []string
		if boolField(m, "staged") {
			parts = append(parts, "--staged")
		}
		if fp := strField(m, "file_path"); fp != "" {
			parts = append(parts, fp)
		}
		return strings.Join(parts, "  ")
	case "git_log":
		var parts []string
		if b := strField(m, "branch"); b != "" {
			parts = append(parts, b)
		}
		if fp := strField(m, "file_path"); fp != "" {
			parts = append(parts, fp)
		}
		return strings.Join(parts, "  ")
	case "git_blame":
		s := strField(m, "file_path")
		if ls := intField(m, "line_start"); ls > 0 {
			s += fmt.Sprintf(":%d", ls)
			if le := intField(m, "line_end"); le > 0 {
				s += fmt.Sprintf("-%d", le)
			}
		}
		return s
	case "git_show":
		s := strField(m, "ref")
		if fp := strField(m, "file_path"); fp != "" {
			s += "  " + fp
		}
		return s
	case "git_checkout":
		s := strField(m, "branch")
		if boolField(m, "create") {
			s += "  (new branch)"
		}
		return s
	case "git_push":
		remote := strField(m, "remote")
		branch := strField(m, "branch")
		s := remote
		if branch != "" {
			if s != "" {
				s += "/" + branch
			} else {
				s = branch
			}
		}
		if boolField(m, "force") {
			s += "  --force"
		}
		return s

	// ── memory ────────────────────────────────────────────────────────────────
	case "memory_read", "memory_write", "memory_edit", "memory_delete":
		s := strField(m, "filename")
		if store := strField(m, "store"); store != "" {
			s += "  @" + store
		}
		return s
	case "memory_list":
		return strField(m, "store")
	case "memory_search":
		s := strField(m, "query")
		if store := strField(m, "store"); store != "" {
			s += "  @" + store
		}
		return s

	// ── skill ─────────────────────────────────────────────────────────────────
	case "skill", "use_skill":
		s := strField(m, "skill")
		if args := strField(m, "args"); args != "" {
			s += "  " + truncate(args, 60)
		}
		return s

	default:
		var parts []string
		for k, v := range m {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			if len(parts) == 3 {
				break
			}
		}
		return truncate(strings.Join(parts, "  "), 120)
	}
}

func strField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func boolField(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func intField(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	f, _ := v.(float64)
	return int(f)
}

func stringsField(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
