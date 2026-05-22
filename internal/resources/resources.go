package resources

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Environment captures the full runtime environment for a session's sandbox:
// packages to pre-install, networking policy, and extra env-var overrides.
type Environment struct {
	Packages   PackageSpec
	Networking NetworkConfig
	Env        map[string]string
}

// FileResource describes a file to materialise in the session workspace.
// Exactly one of Content or SourceURL must be set; neither is stored in DB.
type FileResource struct {
	ID         string // unique resource ID (caller-supplied or generated)
	TargetPath string // path relative to workspace where file is written
	Content    []byte // inline content; mutually exclusive with SourceURL
	SourceURL  string // remote URL; sandbox fetches directly, bypassing orchestration memory
}

// GitResource describes a git repository to clone into the session workspace.
// Token is write-only: passed during add, never stored in DB or returned.
type GitResource struct {
	ID         string // unique resource ID
	URL        string // repository URL
	Branch     string // branch / ref to clone (empty = default branch)
	TargetPath string // path relative to workspace where the repo is cloned
	Token      string // auth token; write-only, not persisted
}

// PackageSpec lists packages to pre-install in a sandbox container.
type PackageSpec struct {
	Pip   []string `json:"pip,omitempty"`
	Npm   []string `json:"npm,omitempty"`
	Apt   []string `json:"apt,omitempty"`
	Cargo []string `json:"cargo,omitempty"`
}

func (s PackageSpec) IsEmpty() bool {
	return len(s.Pip)+len(s.Npm)+len(s.Apt)+len(s.Cargo) == 0
}

const (
	NetworkingUnrestricted = "unrestricted"
	NetworkingLimited      = "limited"
)

// NetworkConfig controls the outbound network policy for a sandbox container.
type NetworkConfig struct {
	Mode         string   // NetworkingUnrestricted (default) | NetworkingLimited
	AllowedHosts []string // only used when Mode == NetworkingLimited
}

// SafeJoin joins root and rel, returning the absolute path and an error if rel
// is empty, absolute, or would escape root via path traversal.
func SafeJoin(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("target_path must not be empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("target_path must be relative, got %q", rel)
	}
	dst := filepath.Join(root, rel)
	if !strings.HasPrefix(dst, root+string(filepath.Separator)) && dst != root {
		return "", fmt.Errorf("target_path %q escapes workspace root", rel)
	}
	return dst, nil
}

// EmbedToken injects a token into an HTTPS git URL.
// "https://github.com/org/repo" → "https://x-token:TOKEN@github.com/org/repo"
func EmbedToken(rawURL, token string) string {
	const prefix = "https://"
	if len(rawURL) > len(prefix) && rawURL[:len(prefix)] == prefix {
		return prefix + "x-token:" + token + "@" + rawURL[len(prefix):]
	}
	return rawURL
}
