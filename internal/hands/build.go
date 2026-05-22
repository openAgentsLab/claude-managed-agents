package hands

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"

	"forge/internal/config"
	"forge/internal/gateway/store"
	mcpclient "forge/internal/mcp/client"
	"forge/internal/reqctx"
	"forge/internal/tools"
	mcptools "forge/internal/tools/mcp"
)

// BuildSandboxLayer sets up the tool registry and per-session sandbox pool for
// serve mode. Platform-level MCP server configs from toolsCfg are injected
// into ctx so the MCP RegisterSource can read them without touching the
// filesystem. tools.Build is called exactly once here.
// repo persists sandbox metadata across worker processes; pass store.Sandboxes().
// resRepo persists dynamic resource declarations; pass store.SessionResources()
// (may be nil for drivers that do not support dynamic mounting).
func BuildSandboxLayer(ctx context.Context, sandbox config.SandboxConfig, toolsCfg config.ToolsConfig, repo store.SandboxRepository, resRepo store.SessionResourceRepository) (tools.ToolRegistry, Pool, func(), error) {
	ctx = mcptools.WithConfig(ctx, convertMCPServers(toolsCfg.MCPServers))
	ctx = reqctx.WithWorkspaceRoot(ctx, sandbox.WorkspaceRoot())

	direct, sandboxed, toolsCleanup := tools.Build(ctx)

	reg, err := NewRegistry(ctx, sandboxed.Tools(), direct.Tools())
	if err != nil {
		toolsCleanup()
		return nil, nil, nil, fmt.Errorf("build registry: %w", err)
	}

	pool, err := OpenPool(ctx, sandbox.DriverOrDefault(), sandbox, PoolDeps{Sandbox: repo, Resources: resRepo, Sandboxed: InvokableTools(sandboxed.Tools())})
	if err != nil {
		toolsCleanup()
		return nil, nil, nil, err
	}
	pool.StartBackground(ctx)
	return reg, pool, toolsCleanup, nil
}

// InvokableTools filters a []tool.BaseTool slice down to tools that implement
// tool.InvokableTool.
func InvokableTools(ts []tool.BaseTool) []tool.InvokableTool {
	out := make([]tool.InvokableTool, 0, len(ts))
	for _, t := range ts {
		if inv, ok := t.(tool.InvokableTool); ok {
			out = append(out, inv)
		}
	}
	return out
}

// convertMCPServers translates config-layer MCP server definitions to the
// mcp/client types used at connection time.
func convertMCPServers(m map[string]config.MCPServerConfig) map[string]mcpclient.MCPServerConfig {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]mcpclient.MCPServerConfig, len(m))
	for name, c := range m {
		out[name] = mcpclient.MCPServerConfig{
			Type:     mcpclient.MCPServerType(c.Type),
			Command:  c.Command,
			Args:     c.Args,
			Env:      c.Env,
			URL:      c.URL,
			Headers:  c.Headers,
			Disabled: c.Disabled,
		}
	}
	return out
}
