package hands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	internaltools "forge/internal/tools"
)

// sandboxTool is a tool.InvokableTool whose execution is proxied through a
// Sandbox injected into ctx. The Info() (name, description, schema) comes from
// the original declaration; InvokableRun() routes the call through the sandbox
// unless inner is non-nil, in which case the tool runs in-process directly.
//
// inner is set for "direct" tools (e.g. MCP) that must execute on the host
// rather than inside a container. This gives all tools a uniform sandboxTool
// wrapper so the permission middleware has a single intercept point.
type sandboxTool struct {
	name  string
	info  *schema.ToolInfo
	inner tool.InvokableTool // non-nil → run in-process, bypass sandbox
}

func (t *sandboxTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *sandboxTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	if t.inner != nil {
		return t.inner.InvokableRun(ctx, args, opts...)
	}
	if sb := SandboxFromContext(ctx); sb != nil {
		return sb.Execute(ctx, t.name, json.RawMessage(args))
	}
	return "", fmt.Errorf("no sandbox available for tool %q", t.name)
}

// sandboxRegistry is a ToolRegistry backed by sandboxTool proxies.
type sandboxRegistry struct {
	tools []tool.BaseTool
}

func (r *sandboxRegistry) Tools() []tool.BaseTool { return r.tools }

// NewRegistry builds a unified ToolRegistry from two sets of tools:
//
//   - sandboxed: routed through the Sandbox injected into ctx at call time
//   - direct:    run in-process on the host (e.g. MCP tools with live connections)
//
// Both sets are wrapped as sandboxTool so the permission middleware has a single
// intercept point regardless of execution environment.
func NewRegistry(ctx context.Context, sandboxed, direct []tool.BaseTool) (internaltools.ToolRegistry, error) {
	proxies := make([]tool.BaseTool, 0, len(sandboxed)+len(direct))

	for _, bt := range sandboxed {
		info, err := bt.Info(ctx)
		if err != nil || info == nil {
			slog.Warn("hands: skipping sandboxed tool with no Info", "err", err)
			continue
		}
		infoCopy := *info
		proxies = append(proxies, &sandboxTool{name: infoCopy.Name, info: &infoCopy})
	}

	for _, bt := range direct {
		info, err := bt.Info(ctx)
		if err != nil || info == nil {
			slog.Warn("hands: skipping direct tool with no Info", "err", err)
			continue
		}
		inv, ok := bt.(tool.InvokableTool)
		if !ok {
			continue
		}
		infoCopy := *info
		proxies = append(proxies, &sandboxTool{name: infoCopy.Name, info: &infoCopy, inner: inv})
	}

	return &sandboxRegistry{tools: proxies}, nil
}

