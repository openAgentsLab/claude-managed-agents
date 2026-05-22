package reqctx

import (
	"context"
	"testing"
)

// ── UserID ────────────────────────────────────────────────────────────────────

func TestUserID_RoundTrip(t *testing.T) {
	ctx := WithUserID(context.Background(), "tenant1/alice")
	if got := UserIDFromContext(ctx); got != "tenant1/alice" {
		t.Errorf("UserIDFromContext: got %q, want %q", got, "tenant1/alice")
	}
}

func TestUserID_EmptyWhenAbsent(t *testing.T) {
	if got := UserIDFromContext(context.Background()); got != "" {
		t.Errorf("expected empty userID, got %q", got)
	}
}

// ── ProjectID ─────────────────────────────────────────────────────────────────

func TestProjectID_RoundTrip(t *testing.T) {
	ctx := WithProjectID(context.Background(), "proj-abc")
	if got := ProjectIDFromContext(ctx); got != "proj-abc" {
		t.Errorf("ProjectIDFromContext: got %q, want %q", got, "proj-abc")
	}
}

func TestProjectID_EmptyWhenAbsent(t *testing.T) {
	if got := ProjectIDFromContext(context.Background()); got != "" {
		t.Errorf("expected empty projectID, got %q", got)
	}
}

// ── TenantID ──────────────────────────────────────────────────────────────────

func TestTenantID_RoundTrip(t *testing.T) {
	ctx := WithTenantID(context.Background(), "tenant-x")
	if got := TenantIDFromContext(ctx); got != "tenant-x" {
		t.Errorf("TenantIDFromContext: got %q, want %q", got, "tenant-x")
	}
}

func TestTenantID_EmptyWhenAbsent(t *testing.T) {
	if got := TenantIDFromContext(context.Background()); got != "" {
		t.Errorf("expected empty tenantID, got %q", got)
	}
}

// ── Role ──────────────────────────────────────────────────────────────────────

func TestRole_RoundTrip(t *testing.T) {
	for _, role := range []string{"admin", "member", "viewer"} {
		ctx := WithRole(context.Background(), role)
		if got := RoleFromContext(ctx); got != role {
			t.Errorf("RoleFromContext(%q): got %q", role, got)
		}
	}
}

func TestRole_EmptyWhenAbsent(t *testing.T) {
	if got := RoleFromContext(context.Background()); got != "" {
		t.Errorf("expected empty role, got %q", got)
	}
}

// ── PermissionMode ────────────────────────────────────────────────────────────

func TestPermissionMode_RoundTrip(t *testing.T) {
	for _, mode := range []string{"default", "plan"} {
		ctx := WithPermissionMode(context.Background(), mode)
		if got := PermissionModeFromContext(ctx); got != mode {
			t.Errorf("PermissionModeFromContext(%q): got %q", mode, got)
		}
	}
}

func TestPermissionMode_EmptyWhenAbsent(t *testing.T) {
	if got := PermissionModeFromContext(context.Background()); got != "" {
		t.Errorf("expected empty permission mode, got %q", got)
	}
}

// ── HITLGate ──────────────────────────────────────────────────────────────────

func TestHITLGate_RoundTrip(t *testing.T) {
	called := false
	gate := HITLGate(func(_ context.Context, _, _ string) bool {
		called = true
		return true
	})
	ctx := WithHITLGate(context.Background(), gate)
	got := HITLGateFromContext(ctx)
	if got == nil {
		t.Fatal("expected non-nil HITLGate")
	}
	result := got(context.Background(), "bash", `{"command":"echo hi"}`)
	if !called {
		t.Error("expected gate to be called")
	}
	if !result {
		t.Error("expected gate to return true")
	}
}

func TestHITLGate_NilWhenAbsent(t *testing.T) {
	if got := HITLGateFromContext(context.Background()); got != nil {
		t.Error("expected nil HITLGate when not set")
	}
}

// ── WorkspaceRoot ─────────────────────────────────────────────────────────────

func TestWorkspaceRoot_RoundTrip(t *testing.T) {
	ctx := WithWorkspaceRoot(context.Background(), "/workspace/proj1")
	if got := WorkspaceRootFromCtx(ctx); got != "/workspace/proj1" {
		t.Errorf("WorkspaceRootFromCtx: got %q, want %q", got, "/workspace/proj1")
	}
}

func TestWorkspaceRoot_FallbackToHome(t *testing.T) {
	// When not set, should fall back to $HOME or "."
	root := WorkspaceRootFromCtx(context.Background())
	if root == "" {
		t.Error("WorkspaceRootFromCtx fallback should not be empty")
	}
}

// ── Key isolation ─────────────────────────────────────────────────────────────

func TestContextKeys_AreIsolated(t *testing.T) {
	ctx := WithUserID(context.Background(), "user1")
	ctx = WithProjectID(ctx, "proj1")
	ctx = WithTenantID(ctx, "tenant1")
	ctx = WithRole(ctx, "admin")
	ctx = WithPermissionMode(ctx, "plan")

	if UserIDFromContext(ctx) != "user1" {
		t.Error("UserID contaminated")
	}
	if ProjectIDFromContext(ctx) != "proj1" {
		t.Error("ProjectID contaminated")
	}
	if TenantIDFromContext(ctx) != "tenant1" {
		t.Error("TenantID contaminated")
	}
	if RoleFromContext(ctx) != "admin" {
		t.Error("Role contaminated")
	}
	if PermissionModeFromContext(ctx) != "plan" {
		t.Error("PermissionMode contaminated")
	}
}
