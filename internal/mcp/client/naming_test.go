package client

import (
	"strings"
	"testing"
)

// ── normalizeForMCP ───────────────────────────────────────────────────────────

func TestNormalizeForMCP_AlphanumericUnchanged(t *testing.T) {
	if got := normalizeForMCP("hello123"); got != "hello123" {
		t.Errorf("normalizeForMCP(%q) = %q, want %q", "hello123", got, "hello123")
	}
}

func TestNormalizeForMCP_AllowedCharsUnchanged(t *testing.T) {
	input := "my-server_name"
	if got := normalizeForMCP(input); got != input {
		t.Errorf("normalizeForMCP(%q) = %q, want %q", input, got, input)
	}
}

func TestNormalizeForMCP_SpaceReplacedWithUnderscore(t *testing.T) {
	got := normalizeForMCP("my server")
	if got != "my_server" {
		t.Errorf("normalizeForMCP(%q) = %q, want %q", "my server", got, "my_server")
	}
}

func TestNormalizeForMCP_SpecialCharsReplaced(t *testing.T) {
	got := normalizeForMCP("my.server/v2@host")
	for _, ch := range got {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
			t.Errorf("normalizeForMCP produced disallowed char %q in %q", ch, got)
		}
	}
}

// ── BuildMCPToolName ──────────────────────────────────────────────────────────

func TestBuildMCPToolName_NormalCase(t *testing.T) {
	got := BuildMCPToolName("myserver", "my_tool")
	want := "mcp__myserver__my_tool"
	if got != want {
		t.Errorf("BuildMCPToolName = %q, want %q", got, want)
	}
}

func TestBuildMCPToolName_SpecialCharsNormalized(t *testing.T) {
	got := BuildMCPToolName("my.server", "list/files")
	if !strings.HasPrefix(got, "mcp__my_server__") {
		t.Errorf("server part not normalized: %q", got)
	}
	if !strings.HasPrefix(got, "mcp__") {
		t.Errorf("result should start with mcp__: %q", got)
	}
}

func TestBuildMCPToolName_WithinMaxLen(t *testing.T) {
	got := BuildMCPToolName("myserver", "my_tool")
	if len(got) > maxToolNameLen {
		t.Errorf("result length %d exceeds max %d: %q", len(got), maxToolNameLen, got)
	}
}

func TestBuildMCPToolName_LongToolNameTruncated(t *testing.T) {
	server := "srv"
	tool := strings.Repeat("a", 100)
	got := BuildMCPToolName(server, tool)
	if len(got) > maxToolNameLen {
		t.Errorf("truncated name length %d exceeds max %d: %q", len(got), maxToolNameLen, got)
	}
	if !strings.HasPrefix(got, "mcp__srv__") {
		t.Errorf("prefix should be preserved: %q", got)
	}
}

func TestBuildMCPToolName_LongToolNameHasSuffix(t *testing.T) {
	// Long tool names should get a hash suffix so different long tools are distinguishable
	tool1 := strings.Repeat("a", 100)
	tool2 := strings.Repeat("b", 100)
	got1 := BuildMCPToolName("srv", tool1)
	got2 := BuildMCPToolName("srv", tool2)
	if got1 == got2 {
		t.Errorf("different long tool names should produce different results: %q == %q", got1, got2)
	}
}

func TestBuildMCPToolName_LongServerNameTruncated(t *testing.T) {
	server := strings.Repeat("s", 100)
	got := BuildMCPToolName(server, "tool")
	if len(got) > maxToolNameLen {
		t.Errorf("degenerate long server: result length %d > %d: %q", len(got), maxToolNameLen, got)
	}
}

func TestBuildMCPToolName_Deterministic(t *testing.T) {
	got1 := BuildMCPToolName("srv", "tool")
	got2 := BuildMCPToolName("srv", "tool")
	if got1 != got2 {
		t.Errorf("BuildMCPToolName should be deterministic: %q != %q", got1, got2)
	}
}

// ── SplitMCPToolName ──────────────────────────────────────────────────────────

func TestSplitMCPToolName_Valid(t *testing.T) {
	server, tool, ok := SplitMCPToolName("mcp__myserver__my_tool")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if server != "myserver" {
		t.Errorf("server = %q, want %q", server, "myserver")
	}
	if tool != "my_tool" {
		t.Errorf("tool = %q, want %q", tool, "my_tool")
	}
}

func TestSplitMCPToolName_NotMCPName(t *testing.T) {
	_, _, ok := SplitMCPToolName("bash")
	if ok {
		t.Error("'bash' should not parse as MCP tool name")
	}
}

func TestSplitMCPToolName_OnlyPrefix(t *testing.T) {
	_, _, ok := SplitMCPToolName("mcp__")
	if ok {
		t.Error("'mcp__' alone (no tool part) should return ok=false")
	}
}

func TestSplitMCPToolName_ToolWithDoubleUnderscore(t *testing.T) {
	// Tool part may contain double underscores; only split on the first occurrence
	server, tool, ok := SplitMCPToolName("mcp__srv__tool__v2")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if server != "srv" {
		t.Errorf("server = %q, want %q", server, "srv")
	}
	if tool != "tool__v2" {
		t.Errorf("tool = %q, want %q", tool, "tool__v2")
	}
}

// ── IsMCPToolName ─────────────────────────────────────────────────────────────

func TestIsMCPToolName_True(t *testing.T) {
	if !IsMCPToolName("mcp__srv__tool") {
		t.Error("mcp__srv__tool should be an MCP tool name")
	}
}

func TestIsMCPToolName_False(t *testing.T) {
	for _, name := range []string{"bash", "read_file", "write_file", "mcp__only"} {
		if IsMCPToolName(name) {
			t.Errorf("%q should not be an MCP tool name", name)
		}
	}
}

// ── simpleHash ────────────────────────────────────────────────────────────────

func TestSimpleHash_Deterministic(t *testing.T) {
	h1 := simpleHash("hello")
	h2 := simpleHash("hello")
	if h1 != h2 {
		t.Error("simpleHash should be deterministic")
	}
}

func TestSimpleHash_DifferentInputs(t *testing.T) {
	h1 := simpleHash("hello")
	h2 := simpleHash("world")
	if h1 == h2 {
		t.Error("different inputs should (almost always) produce different hashes")
	}
}
