// Package policy defines MCP tool risk levels and allowlists.
//
// Mirrors forge's filterMcpServersByPolicy logic.
// Supports three match modes for both allow and deny lists:
//   - server name (exact, case-insensitive)
//   - server command  (stdio only, exact match on the command field)
//   - server URL      (remote only, supports * wildcard)
package policy

import (
	"strings"

	mcpclient "forge/internal/mcp/client"
)

// Settings holds the allow/deny configuration for MCP servers.
// Deny takes priority over allow.
type Settings struct {
	// AllowedServers is a whitelist.  When non-empty, only listed servers are
	// permitted.  Each entry may be a server name, a command path, or a URL
	// pattern (with optional * wildcards).
	AllowedServers []string `json:"allowedMcpServers,omitempty"`
	// DeniedServers is a blacklist.  Listed servers are always blocked,
	// regardless of AllowedServers.  Same matching rules as AllowedServers.
	DeniedServers []string `json:"deniedMcpServers,omitempty"`
}

// Filter removes servers that are denied or not in the allowlist.
// Returns a new map with only the permitted entries.
func Filter(servers map[string]mcpclient.MCPServerConfig, ps Settings) map[string]mcpclient.MCPServerConfig {
	out := make(map[string]mcpclient.MCPServerConfig, len(servers))
	for name, cfg := range servers {
		if matchesAny(name, cfg, ps.DeniedServers) {
			continue
		}
		if len(ps.AllowedServers) > 0 && !matchesAny(name, cfg, ps.AllowedServers) {
			continue
		}
		out[name] = cfg
	}
	return out
}

// matchesAny reports whether the server (name + config) matches at least one
// pattern in patterns.  Patterns are matched against:
//  1. Server name (case-insensitive exact match)
//  2. Command field for stdio servers (case-insensitive)
//  3. URL field for remote servers (supports * wildcard, case-insensitive)
func matchesAny(name string, cfg mcpclient.MCPServerConfig, patterns []string) bool {
	for _, p := range patterns {
		if matchesOne(name, cfg, p) {
			return true
		}
	}
	return false
}

func matchesOne(name string, cfg mcpclient.MCPServerConfig, pattern string) bool {
	lower := strings.ToLower(pattern)

	// 1. Name match (exact, case-insensitive).
	if strings.EqualFold(name, pattern) {
		return true
	}

	typ := cfg.Type
	if typ == "" {
		typ = mcpclient.MCPStdio
	}

	switch typ {
	case mcpclient.MCPStdio:
		// 2. Command match (case-insensitive exact).
		if cfg.Command != "" && strings.EqualFold(cfg.Command, pattern) {
			return true
		}
	default:
		// 3. URL match with * wildcard support.
		if cfg.URL != "" && globMatch(lower, strings.ToLower(cfg.URL)) {
			return true
		}
	}
	return false
}

// globMatch reports whether pattern (which may contain * wildcards) matches s.
// Both arguments should be lower-cased before calling.
// Mirrors forge's URL wildcard matching semantics.
func globMatch(pattern, s string) bool {
	// Split pattern on * and match each segment in order.
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}

	// First segment must match the start of s.
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]

	// Middle segments must appear in order.
	for _, part := range parts[1 : len(parts)-1] {
		idx := strings.Index(s, part)
		if idx < 0 {
			return false
		}
		s = s[idx+len(part):]
	}

	// Last segment must match the end of s.
	last := parts[len(parts)-1]
	return strings.HasSuffix(s, last)
}
