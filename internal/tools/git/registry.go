package git

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"

	"forge/internal/reqctx"
)

// GitRegistry holds the git tool instances for a workspace.
// It covers git_status, git_diff, git_log, git_blame, git_add,
// git_commit, git_checkout, git_show, and git_push.
type GitRegistry struct {
	tools []tool.BaseTool
}

// NewGitRegistry creates a GitRegistry for the given workspace.
// Returns an empty registry (no error) if git is not installed.
func NewGitRegistry(ctx context.Context) (*GitRegistry, error) {
	root := reqctx.WorkspaceRootFromCtx(ctx)
	gitTools, err := NewGitTools(root)
	if err != nil {
		return nil, fmt.Errorf("git registry: %w", err)
	}
	return &GitRegistry{tools: gitTools}, nil
}

// Tools implements tools.ToolRegistry.
func (r *GitRegistry) Tools() []tool.BaseTool { return r.tools }
