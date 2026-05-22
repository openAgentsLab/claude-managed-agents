package fs

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"

	"forge/internal/reqctx"
)

// FsRegistry holds the filesystem tool instances for a workspace.
// It covers read_file, write_file, list_dir, glob, grep_file, file_edit,
// delete_file, delete_dir, and move_file.
// Command execution tools (bash) live in tools/exec.ExecRegistry.
type FsRegistry struct {
	tools []tool.BaseTool
}

// NewFsRegistry creates a FsRegistry for the given workspace.
func NewFsRegistry(ctx context.Context) (*FsRegistry, error) {
	root := reqctx.WorkspaceRootFromCtx(ctx)
	wsTools, err := NewTools(root, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("fs registry: %w", err)
	}
	return &FsRegistry{tools: wsTools}, nil
}

// Tools implements tools.ToolRegistry.
func (r *FsRegistry) Tools() []tool.BaseTool { return r.tools }
