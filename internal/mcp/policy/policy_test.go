package policy

import (
	"testing"

	mcpclient "forge/internal/mcp/client"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func stdioServer(command string) mcpclient.MCPServerConfig {
	return mcpclient.MCPServerConfig{Type: mcpclient.MCPStdio, Command: command}
}

func remoteServer(url string) mcpclient.MCPServerConfig {
	return mcpclient.MCPServerConfig{Type: "sse", URL: url}
}

// ── Filter ────────────────────────────────────────────────────────────────────

func TestFilter_Empty_AllowsAll(t *testing.T) {
	servers := map[string]mcpclient.MCPServerConfig{
		"fs":  stdioServer("/usr/bin/mcp-fs"),
		"git": stdioServer("/usr/bin/mcp-git"),
	}
	out := Filter(servers, Settings{})
	if len(out) != 2 {
		t.Errorf("expected 2 servers, got %d", len(out))
	}
}

func TestFilter_DenyByName(t *testing.T) {
	servers := map[string]mcpclient.MCPServerConfig{
		"fs":  stdioServer("/usr/bin/mcp-fs"),
		"git": stdioServer("/usr/bin/mcp-git"),
	}
	out := Filter(servers, Settings{DeniedServers: []string{"fs"}})
	if _, ok := out["fs"]; ok {
		t.Error("denied server 'fs' should be filtered out")
	}
	if _, ok := out["git"]; !ok {
		t.Error("allowed server 'git' should remain")
	}
}

func TestFilter_AllowByName_BlocksOthers(t *testing.T) {
	servers := map[string]mcpclient.MCPServerConfig{
		"fs":  stdioServer("/usr/bin/mcp-fs"),
		"git": stdioServer("/usr/bin/mcp-git"),
		"web": remoteServer("https://web.example.com"),
	}
	out := Filter(servers, Settings{AllowedServers: []string{"git"}})
	if _, ok := out["git"]; !ok {
		t.Error("allowed server 'git' should pass")
	}
	if _, ok := out["fs"]; ok {
		t.Error("non-allowed server 'fs' should be blocked")
	}
	if _, ok := out["web"]; ok {
		t.Error("non-allowed server 'web' should be blocked")
	}
}

func TestFilter_DenyOverridesAllow(t *testing.T) {
	servers := map[string]mcpclient.MCPServerConfig{
		"fs": stdioServer("/usr/bin/mcp-fs"),
	}
	ps := Settings{
		AllowedServers: []string{"fs"},
		DeniedServers:  []string{"fs"},
	}
	out := Filter(servers, ps)
	if _, ok := out["fs"]; ok {
		t.Error("deny should override allow")
	}
}

func TestFilter_NameMatchCaseInsensitive(t *testing.T) {
	servers := map[string]mcpclient.MCPServerConfig{
		"MyServer": stdioServer("/bin/mcp"),
	}
	out := Filter(servers, Settings{DeniedServers: []string{"myserver"}})
	if _, ok := out["MyServer"]; ok {
		t.Error("name match should be case-insensitive")
	}
}

func TestFilter_CommandMatch(t *testing.T) {
	servers := map[string]mcpclient.MCPServerConfig{
		"dangerous": stdioServer("/usr/local/bin/evil-mcp"),
		"safe":      stdioServer("/usr/local/bin/safe-mcp"),
	}
	out := Filter(servers, Settings{DeniedServers: []string{"/usr/local/bin/evil-mcp"}})
	if _, ok := out["dangerous"]; ok {
		t.Error("server matching denied command should be filtered")
	}
	if _, ok := out["safe"]; !ok {
		t.Error("server with different command should remain")
	}
}

func TestFilter_URLWildcard(t *testing.T) {
	servers := map[string]mcpclient.MCPServerConfig{
		"prod":    remoteServer("https://api.example.com/mcp"),
		"staging": remoteServer("https://staging.example.com/mcp"),
		"other":   remoteServer("https://other.io/mcp"),
	}
	out := Filter(servers, Settings{AllowedServers: []string{"*.example.com*"}})
	if _, ok := out["prod"]; !ok {
		t.Error("prod (api.example.com) should match *.example.com*")
	}
	if _, ok := out["staging"]; !ok {
		t.Error("staging (staging.example.com) should match *.example.com*")
	}
	if _, ok := out["other"]; ok {
		t.Error("other (other.io) should not match *.example.com*")
	}
}

func TestFilter_EmptyServers(t *testing.T) {
	out := Filter(map[string]mcpclient.MCPServerConfig{}, Settings{DeniedServers: []string{"anything"}})
	if len(out) != 0 {
		t.Errorf("expected empty output for empty input, got %d", len(out))
	}
}

// ── globMatch ─────────────────────────────────────────────────────────────────

func TestGlobMatch_Exact(t *testing.T) {
	if !globMatch("hello", "hello") {
		t.Error("exact match should return true")
	}
	if globMatch("hello", "world") {
		t.Error("non-matching exact should return false")
	}
}

func TestGlobMatch_StarAnywhere(t *testing.T) {
	if !globMatch("*.example.com", "api.example.com") {
		t.Error("*.example.com should match api.example.com")
	}
	if globMatch("*.example.com", "example.com") {
		t.Error("*.example.com should not match example.com (no subdomain)")
	}
}

func TestGlobMatch_StarMiddle(t *testing.T) {
	if !globMatch("https://*.com/path", "https://example.com/path") {
		t.Error("star in middle should match")
	}
}

func TestGlobMatch_MultipleStars(t *testing.T) {
	if !globMatch("*example*mcp*", "https://example.io/mcp/v1") {
		t.Error("multiple stars should match")
	}
}

func TestGlobMatch_StarMatchesEmpty(t *testing.T) {
	if !globMatch("prefix*suffix", "prefixsuffix") {
		t.Error("star should match empty string between parts")
	}
}

func TestGlobMatch_OnlyStar(t *testing.T) {
	if !globMatch("*", "anything-goes-here") {
		t.Error("bare * should match anything")
	}
}

func TestGlobMatch_EmptyPattern(t *testing.T) {
	if !globMatch("", "") {
		t.Error("empty pattern should match empty string")
	}
	if globMatch("", "nonempty") {
		t.Error("empty pattern should not match non-empty string")
	}
}
