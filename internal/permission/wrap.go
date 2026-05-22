package permission

import (
	"context"

	einotool "github.com/cloudwego/eino/components/tool"

	"forge/internal/reqctx"
	"forge/internal/tools"
	"forge/internal/tools/middleware"
)

// NewTenantEngineResolver returns an EngineResolver that selects the Engine
// for the tenant identified in ctx.
//
// Engine selection order:
//  1. If role == "viewer" → use "{tenantID}:viewer" engine (always plan mode).
//  2. Use "{tenantID}" engine for the tenant.
//  3. Fall back to "default" engine.
//
// Falls back to a fresh ModeDefault engine if none of the above match.
func NewTenantEngineResolver(engines map[string]*Engine) EngineResolver {
	return func(ctx context.Context) *Engine {
		tid := reqctx.TenantIDFromContext(ctx)
		role := reqctx.RoleFromContext(ctx)

		// Viewer role is always restricted to plan (read-only) mode.
		if role == "viewer" {
			if e, ok := engines[tid+":viewer"]; ok {
				return e
			}
		}

		if e, ok := engines[tid]; ok {
			return e
		}
		if e, ok := engines["default"]; ok {
			return e
		}
		// Defensive fallback: return any engine rather than nil.
		for _, e := range engines {
			return e
		}
		return NewEngine(ModeDefault)
	}
}

// WrapRegistryWithResolver wraps every tool in reg with a resolver-driven
// permission interceptor and returns the new registry.
//
// Used by serve mode where each request may belong to a different tenant.
// audit may be nil to disable audit logging.
func WrapRegistryWithResolver(
	reg tools.ToolRegistry,
	resolver EngineResolver,
	readOnly map[string]bool,
	audit AuditLogger,
) tools.ToolRegistry {
	intercept := NewInterceptor(resolver, readOnly, audit)
	return wrapTools(reg, intercept)
}

func wrapTools(reg tools.ToolRegistry, intercept middleware.Interceptor) tools.ToolRegistry {
	ts := reg.Tools()
	wrapped := make([]einotool.BaseTool, 0, len(ts))
	for _, t := range ts {
		if inv, ok := t.(einotool.InvokableTool); ok {
			wrapped = append(wrapped, middleware.Wrap(inv, intercept, middleware.SafeError()))
		} else {
			wrapped = append(wrapped, t)
		}
	}
	return tools.Static(wrapped)
}
