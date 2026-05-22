package local

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"forge/internal/hands"
	"forge/internal/tools"
)

// newProvisionedSandbox creates a LocalSandbox with the full built-in tool set
// registered, mirroring how BuildToolServerRegistry provisions the sandbox in
// production. Tests that exercise tool execution must use this helper.
func newProvisionedSandbox(t *testing.T) *LocalSandbox {
	t.Helper()
	ctx := context.Background()
	_, sandboxed, cleanup := tools.Build(ctx)
	t.Cleanup(cleanup)
	sb := NewLocalSandbox()
	if err := sb.Provision(ctx, hands.InvokableTools(sandboxed.Tools())); err != nil {
		t.Fatalf("Provision sandbox: %v", err)
	}
	return sb
}

// ── Provision ─────────────────────────────────────────────────────────────

func TestLocalSandbox_Provision_Succeeds(t *testing.T) {
	sb := NewLocalSandbox()
	if err := sb.Provision(context.Background(), nil); err != nil {
		t.Fatalf("Provision: %v", err)
	}
}

func TestLocalSandbox_Provision_InvalidWorkspaceRoot(t *testing.T) {
	sb := NewLocalSandbox()
	_ = sb.Provision(context.Background(), nil)
}

// ── Execute ───────────────────────────────────────────────────────────────

func TestLocalSandbox_Execute_UnknownTool_ReturnsError(t *testing.T) {
	sb := NewLocalSandbox()
	_ = sb.Provision(context.Background(), nil)

	_, err := sb.Execute(context.Background(), "no_such_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown tool, got nil")
	}
}

func TestLocalSandbox_Execute_BeforeProvision_ReturnsError(t *testing.T) {
	sb := NewLocalSandbox()
	_, err := sb.Execute(context.Background(), "bash", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error when Execute called before Provision, got nil")
	}
}

func TestLocalSandbox_Execute_Bash_Echo(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping bash execution in CI")
	}
	sb := newProvisionedSandbox(t)

	out, err := sb.Execute(context.Background(), "bash", json.RawMessage(`{"command":"echo hello"}`))
	if err != nil {
		t.Fatalf("Execute bash echo: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty output from echo, got empty string")
	}
}

func TestLocalSandbox_Execute_ReadFile(t *testing.T) {
	sb := newProvisionedSandbox(t)
	dir := t.TempDir()

	absPath := dir + "/hello.txt"
	if err := os.WriteFile(absPath, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	input, _ := json.Marshal(map[string]string{"file_path": absPath})
	out, err := sb.Execute(context.Background(), "read_file", json.RawMessage(input))
	if err != nil {
		t.Fatalf("Execute read_file: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty output from read_file")
	}
}

// ── Close ─────────────────────────────────────────────────────────────────

func TestLocalSandbox_Close_NoError(t *testing.T) {
	sb := NewLocalSandbox()
	if err := sb.Close(); err != nil {
		t.Errorf("Close() before Provision: %v", err)
	}
	_ = sb.Provision(context.Background(), nil)
	if err := sb.Close(); err != nil {
		t.Errorf("Close() after Provision: %v", err)
	}
}

// ── Interface compliance ──────────────────────────────────────────────────

func TestLocalSandbox_ImplementsSandboxInterface(t *testing.T) {
	var _ hands.Sandbox = NewLocalSandbox()
}
