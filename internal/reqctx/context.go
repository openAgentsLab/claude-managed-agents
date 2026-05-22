// Package reqctx holds request-scoped context keys shared across orchestration,
// tools, and backends. It has no forge dependencies, keeping it import-cycle-free.
package reqctx

import (
	"context"
	"os"
)

type contextKey int

const (
	keyUserID         contextKey = iota
	keyProjectID      contextKey = iota
	keyTenantID       contextKey = iota
	keyRole           contextKey = iota
	keyPermissionMode contextKey = iota
)

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, keyUserID, userID)
}

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(keyUserID).(string)
	return v
}

func WithProjectID(ctx context.Context, projectID string) context.Context {
	return context.WithValue(ctx, keyProjectID, projectID)
}

func ProjectIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(keyProjectID).(string)
	return v
}

func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, keyTenantID, tenantID)
}

func TenantIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(keyTenantID).(string)
	return v
}

// WithRole stores the authenticated user's role in ctx.
// Values: "admin" | "member" | "viewer".
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, keyRole, role)
}

// RoleFromContext returns the role stored by WithRole, or "" if not set.
func RoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(keyRole).(string)
	return v
}

// WithPermissionMode stores a per-request permission mode override in ctx.
// Valid values: "default" | "plan". Empty string means no override.
func WithPermissionMode(ctx context.Context, mode string) context.Context {
	return context.WithValue(ctx, keyPermissionMode, mode)
}

// PermissionModeFromContext returns the per-request mode stored by
// WithPermissionMode, or "" if not set.
func PermissionModeFromContext(ctx context.Context) string {
	v, _ := ctx.Value(keyPermissionMode).(string)
	return v
}

// HITLGate is a function that blocks until the user confirms or denies
// a tool call. Returns true when the user confirms, false when denied or ctx cancelled.
// Injected by the harness into the run context; consumed by the permission interceptor.
type HITLGate func(ctx context.Context, toolName, argsJSON string) bool

type hitlGateKey struct{}

// WithHITLGate injects a HITLGate into ctx.
func WithHITLGate(ctx context.Context, g HITLGate) context.Context {
	return context.WithValue(ctx, hitlGateKey{}, g)
}

// HITLGateFromContext retrieves the HITLGate injected by WithHITLGate, or nil.
func HITLGateFromContext(ctx context.Context) HITLGate {
	g, _ := ctx.Value(hitlGateKey{}).(HITLGate)
	return g
}

type workspaceRootKey struct{}

// WithWorkspaceRoot injects the workspace root into ctx so that tool registries
// and sandbox pool drivers can read the correct construction-time root.
// For isolated pools (Docker/K8s) this is the host-side VolumesRoot; the
// container always mounts it at /workspace and tools do not need to read this.
// For the local (in-process) driver tools use this value directly.
func WithWorkspaceRoot(ctx context.Context, root string) context.Context {
	return context.WithValue(ctx, workspaceRootKey{}, root)
}

// WorkspaceRootFromCtx returns the workspace root injected by WithWorkspaceRoot,
// falling back to $HOME (then ".") when not set.
func WorkspaceRootFromCtx(ctx context.Context) string {
	if r, _ := ctx.Value(workspaceRootKey{}).(string); r != "" {
		return r
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}
