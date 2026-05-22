package client

import (
	"os"
	"regexp"
	"strings"
)

// reEnvVar matches ${VAR} and ${VAR:-default} patterns (like forge's envExpansion.ts).
var reEnvVar = regexp.MustCompile(`\$\{([^}]+)\}`)

// expandEnvVarsFull expands ${VAR} and ${VAR:-default} in s.
// It returns the expanded string and a deduplicated list of variable names
// that were referenced but not set (no default provided).
//
// This is a Go port of forge's expandEnvVarsInString().
func expandEnvVarsFull(s string) (expanded string, missingVars []string) {
	seen := make(map[string]bool)

	result := reEnvVar.ReplaceAllStringFunc(s, func(match string) string {
		inner := match[2 : len(match)-1] // strip ${ and }

		var varName, defaultVal string
		var hasDefault bool

		if idx := strings.Index(inner, ":-"); idx >= 0 {
			varName = inner[:idx]
			defaultVal = inner[idx+2:]
			hasDefault = true
		} else {
			varName = inner
		}

		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		if hasDefault {
			return defaultVal
		}

		if !seen[varName] {
			seen[varName] = true
			missingVars = append(missingVars, varName)
		}
		return match // leave unexpanded so caller can diagnose
	})

	// Also expand bare $VAR (no braces) via os.ExpandEnv for backwards compat.
	// Do this on the already-processed string so ${...} matches aren't re-processed.
	result = os.ExpandEnv(result)
	return result, missingVars
}

// expandEnvString is a convenience wrapper that discards missingVars.
func expandEnvString(s string) string {
	v, _ := expandEnvVarsFull(s)
	return v
}

// expandEnvSliceFull expands each element and aggregates missing vars.
func expandEnvSliceFull(ss []string) ([]string, []string) {
	out := make([]string, len(ss))
	var missing []string
	for i, s := range ss {
		v, m := expandEnvVarsFull(s)
		out[i] = v
		missing = append(missing, m...)
	}
	return out, dedup(missing)
}

// expandEnvMapFull expands map values and aggregates missing vars.
func expandEnvMapFull(m map[string]string) (map[string]string, []string) {
	if m == nil {
		return nil, nil
	}
	out := make(map[string]string, len(m))
	var missing []string
	for k, v := range m {
		expanded, miss := expandEnvVarsFull(v)
		out[k] = expanded
		missing = append(missing, miss...)
	}
	return out, dedup(missing)
}

func dedup(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := ss[:0]
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
