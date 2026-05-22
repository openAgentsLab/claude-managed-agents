package client

import (
	"fmt"
	"regexp"
	"strings"
)

// maxToolNameLen is the maximum length allowed by the Anthropic API for tool
// names: ^[a-zA-Z0-9_-]{1,64}$
const maxToolNameLen = 64

var invalidChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// normalizeForMCP replaces characters outside [a-zA-Z0-9_-] with underscores,
// matching forge's normalizeNameForMCP in normalization.ts.
func normalizeForMCP(name string) string {
	return invalidChars.ReplaceAllString(name, "_")
}

// BuildMCPToolName returns the canonical mcp__<server>__<tool> name used to
// expose MCP tools to the LLM.
//
// If the resulting name would exceed maxToolNameLen (64) characters, the tool
// part is truncated to fit within the limit, preserving the "mcp__<server>__"
// prefix so server routing still works.
func BuildMCPToolName(serverName, toolName string) string {
	s := normalizeForMCP(serverName)
	t := normalizeForMCP(toolName)
	full := "mcp__" + s + "__" + t
	if len(full) <= maxToolNameLen {
		return full
	}

	// Prefix "mcp__<server>__" must be kept intact for routing.
	prefix := "mcp__" + s + "__"
	if len(prefix) >= maxToolNameLen {
		// Degenerate: server name alone is too long.  Hard-truncate everything.
		return full[:maxToolNameLen]
	}

	// Suffix: hash the original tool name to a 8-char hex suffix so different
	// tools that share the same truncated prefix remain distinguishable.
	available := maxToolNameLen - len(prefix) - 9 // 9 = 1 underscore + 8 hex chars
	truncated := t
	if available > 0 && len(t) > available {
		truncated = t[:available]
	} else if available <= 0 {
		truncated = ""
	}
	h := simpleHash(toolName)
	return prefix + truncated + fmt.Sprintf("_%08x", h)
}

// simpleHash returns a stable 32-bit hash of s (djb2).
func simpleHash(s string) uint32 {
	var h uint32 = 5381
	for _, b := range []byte(s) {
		h = ((h << 5) + h) + uint32(b)
	}
	return h
}

// SplitMCPToolName parses a normalised mcp__server__tool name back into its
// components.  It returns ok=false for non-MCP names.
func SplitMCPToolName(name string) (server, tool string, ok bool) {
	if !strings.HasPrefix(name, "mcp__") {
		return "", "", false
	}
	parts := strings.SplitN(name[5:], "__", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// IsMCPToolName reports whether name is in the mcp__…__… format.
func IsMCPToolName(name string) bool {
	_, _, ok := SplitMCPToolName(name)
	return ok
}
