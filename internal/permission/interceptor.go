package permission

import (
	"context"
	"fmt"

	"forge/internal/reqctx"
	"forge/internal/tools/middleware"
)

const (
	// truncateDenyPreview is the max argsJSON length shown in a deny message.
	truncateDenyPreview = 120
)

// EngineResolver selects the permission Engine for an incoming request.
// In single-engine mode it always returns the same Engine.
// In multi-tenant serve mode it reads TenantID from ctx and looks up a map.
type EngineResolver func(ctx context.Context) *Engine

func truncateDisplay(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// NewInterceptor returns a middleware.Interceptor that enforces the permission
// engine returned by resolve before each tool invocation.
//
// Unmatched tools are allowed by default (default-allow semantics).
// Only explicit deny rules or dangerous-command detection will block execution.
// audit may be nil, in which case permission decisions are not logged.
func NewInterceptor(
	resolve EngineResolver,
	readOnly map[string]bool,
	audit AuditLogger,
) middleware.Interceptor {
	return func(ctx context.Context, req *middleware.Request, handler middleware.Handler) (*middleware.Response, error) {
		engine := resolve(ctx)
		toolName := req.Meta.Name
		isReadOnly := readOnly[toolName]
		decision := engine.Check(toolName, req.ArgsJSON, isReadOnly)

		if audit != nil {
			audit.Log(newAuditEvent(ctx, toolName, req.ArgsJSON, decision))
		}

		switch decision.Behavior {
		case BehaviorAllow:
			return handler(ctx, req)

		case BehaviorDeny:
			preview := truncateDisplay(req.ArgsJSON, truncateDenyPreview)
			return &middleware.Response{
				Output: fmt.Sprintf("[permission denied] %s(%s): %s", toolName, preview, decision.Message),
			}, nil

		case BehaviorAsk:
			// Human-in-the-loop: pause and wait for user confirmation.
			// Requires a HITLGate injected into ctx by the harness layer.
			// If no gate is present (e.g. REPL mode), fall back to deny.
			gate := reqctx.HITLGateFromContext(ctx)
			if gate == nil || !gate(ctx, toolName, req.ArgsJSON) {
				preview := truncateDisplay(req.ArgsJSON, truncateDenyPreview)
				return &middleware.Response{
					Output: fmt.Sprintf("[permission denied] %s(%s): user did not confirm", toolName, preview),
				}, nil
			}
			return handler(ctx, req)

		default:
			return nil, fmt.Errorf("unexpected permission decision %q for tool %s", decision.Behavior, toolName)
		}
	}
}
