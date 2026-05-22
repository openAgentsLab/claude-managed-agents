package local

import (
	"context"
	"fmt"

	"forge/internal/hands"
	"forge/internal/tools"
)

// BuildToolServerRegistry builds the sandbox and tool registry for the
// in-container tool-server binary. The returned cleanup must be called on shutdown.
// No platform MCP servers are injected — tool-server runs inside the container
// and should not open outbound MCP connections.
func BuildToolServerRegistry(ctx context.Context) (hands.Sandbox, tools.ToolRegistry, func(), error) {
	direct, sandboxed, toolsCleanup := tools.Build(ctx)

	sb := NewLocalSandbox()
	if err := sb.Provision(ctx, hands.InvokableTools(sandboxed.Tools())); err != nil {
		toolsCleanup()
		return nil, nil, nil, fmt.Errorf("provision sandbox: %w", err)
	}
	cleanup := func() { _ = sb.Close(); toolsCleanup() }

	reg, err := hands.NewRegistry(ctx, sandboxed.Tools(), direct.Tools())
	if err != nil {
		cleanup()
		return nil, nil, nil, fmt.Errorf("build registry: %w", err)
	}
	return sb, reg, cleanup, nil
}
