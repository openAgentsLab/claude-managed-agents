package exec

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"

	"forge/internal/reqctx"
)

// ExecRegistry holds command execution tools (bash, etc.).
// It is registered as a sandboxed source via tools/builtin.go so that
// in Docker mode these tools are routed through DockerSandbox.Execute rather
// than running in the agent process.
type ExecRegistry struct {
	tools []tool.BaseTool
}

// NewExecRegistry creates an ExecRegistry for the given workspace.
func NewExecRegistry(ctx context.Context) (*ExecRegistry, error) {
	root := reqctx.WorkspaceRootFromCtx(ctx)
	execTools, err := NewExecTools(root, "")
	if err != nil {
		return nil, fmt.Errorf("exec registry: %w", err)
	}
	return &ExecRegistry{tools: execTools}, nil
}

// Tools implements tools.ToolRegistry.
func (r *ExecRegistry) Tools() []tool.BaseTool { return r.tools }
