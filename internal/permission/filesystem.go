package permission

import (
	"path/filepath"
	"strings"
)

// protectedPaths lists file/directory names (lowercased, slash-normalised) that
// must never be overwritten by LLM-driven write tools, even when the user has
// broad allow rules in place.
//
// Rationale: an LLM that can modify .forge/settings.json can bypass its own
// permission rules in the next run.  Similarly, modifying .git/hooks can
// execute arbitrary code on the next git operation.
var protectedPaths = []string{
	// Agent / Forge config
	".agent/",
	".forge/",
	// Git internals (hooks, config)
	".git/",
	// SSH keys and config
	".ssh/",
	// Shell initialisation files (by suffix — checked separately)
}

// protectedSuffixes lists filename suffixes (lowercased) that are protected
// regardless of the directory they appear in.
var protectedSuffixes = []string{
	".bashrc",
	".zshrc",
	".profile",
	".bash_profile",
	".bash_login",
	".zprofile",
	".zlogin",
	".zlogout",
	".tcshrc",
	".cshrc",
	".netrc",
	".gitconfig",
	".gitattributes",
}

// IsProtectedPath reports whether filePath refers to a file or directory that
// the LLM must not overwrite.
//
// Matching is case-insensitive and path-separator-normalised so that
// ".Agent/settings.json" on a case-insensitive filesystem cannot bypass
// ".agent/" checks.
func IsProtectedPath(filePath string) bool {
	if filePath == "" {
		return false
	}

	clean := strings.ToLower(filepath.ToSlash(filepath.Clean(filePath)))

	for _, pfx := range protectedPaths {
		if strings.Contains(clean+"/", "/"+pfx) || strings.HasPrefix(clean, pfx) {
			return true
		}
	}

	base := strings.ToLower(filepath.Base(filePath))
	for _, suf := range protectedSuffixes {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}

	return false
}

// ProtectedPathMessage returns a human-readable denial message for a protected
// path, suitable for inclusion in a [permission denied] response.
func ProtectedPathMessage(filePath string) string {
	return "write to protected path not allowed: " + filePath +
		" (modifying agent config, git internals or shell rc files is blocked for safety)"
}
