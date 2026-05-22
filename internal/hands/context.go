package hands

import "context"

type sandboxContextKey struct{}

// WithSandbox stores a per-request Sandbox in ctx. Orchestration injects this
// for each user request in Docker serve mode; sandboxTool checks it first so
// every tool call routes to the user's own container.
func WithSandbox(ctx context.Context, sb Sandbox) context.Context {
	return context.WithValue(ctx, sandboxContextKey{}, sb)
}

// SandboxFromContext returns the per-request Sandbox stored by WithSandbox,
// or nil if none was set (tools fall back to their startup-time sandbox).
func SandboxFromContext(ctx context.Context) Sandbox {
	sb, _ := ctx.Value(sandboxContextKey{}).(Sandbox)
	return sb
}
