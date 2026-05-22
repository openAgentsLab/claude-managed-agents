package git

import (
	"context"
	"os/exec"
	"testing"

	"forge/internal/reqctx"
)

// gitAvailable reports whether git is installed on this machine.
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// ── NewGitRegistry ────────────────────────────────────────────────────────────

func TestNewGitRegistry_NoError(t *testing.T) {
	ctx := reqctx.WithWorkspaceRoot(context.Background(), t.TempDir())
	_, err := NewGitRegistry(ctx)
	if err != nil {
		t.Fatalf("NewGitRegistry: %v", err)
	}
}

func TestNewGitRegistry_ReturnsNineTools(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed — skipping")
	}
	ctx := reqctx.WithWorkspaceRoot(context.Background(), t.TempDir())
	reg, err := NewGitRegistry(ctx)
	if err != nil {
		t.Fatalf("NewGitRegistry: %v", err)
	}
	tools := reg.Tools()
	if len(tools) != 9 {
		t.Errorf("expected 9 git tools, got %d", len(tools))
	}
}

func TestNewGitRegistry_ToolNames(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed — skipping")
	}
	ctx := reqctx.WithWorkspaceRoot(context.Background(), t.TempDir())
	reg, err := NewGitRegistry(ctx)
	if err != nil {
		t.Fatalf("NewGitRegistry: %v", err)
	}
	nameSet := make(map[string]bool)
	for _, bt := range reg.Tools() {
		info, err := bt.Info(context.Background())
		if err != nil || info == nil {
			t.Errorf("tool Info() error: %v", err)
			continue
		}
		nameSet[info.Name] = true
	}
	expectedTools := []string{
		"git_status", "git_diff", "git_log", "git_blame",
		"git_add", "git_commit", "git_checkout", "git_show", "git_push",
	}
	for _, name := range expectedTools {
		if !nameSet[name] {
			t.Errorf("expected git tool %q to be registered", name)
		}
	}
}

// ── NewGitTools constants ─────────────────────────────────────────────────────

func TestGitConstants(t *testing.T) {
	if maxGitOutputBytes <= 0 {
		t.Error("maxGitOutputBytes should be positive")
	}
	if defaultGitTimeout <= 0 {
		t.Error("defaultGitTimeout should be positive")
	}
}

// ── ToolInfo schema ───────────────────────────────────────────────────────────

func TestGitTools_InfoNonNil(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed — skipping")
	}
	ctx := reqctx.WithWorkspaceRoot(context.Background(), t.TempDir())
	reg, err := NewGitRegistry(ctx)
	if err != nil {
		t.Fatalf("NewGitRegistry: %v", err)
	}
	for _, bt := range reg.Tools() {
		info, err := bt.Info(context.Background())
		if err != nil {
			t.Errorf("tool.Info() error: %v", err)
			continue
		}
		if info == nil {
			t.Error("tool.Info() returned nil")
			continue
		}
		if info.Name == "" {
			t.Error("tool info has empty name")
		}
		if info.Desc == "" {
			t.Errorf("tool %q has empty description", info.Name)
		}
	}
}
