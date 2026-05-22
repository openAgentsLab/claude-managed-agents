package git

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type gitStatusRequest struct{}

func newGitStatusTool(workdir string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"git_status",
		"Show the working tree status in short porcelain format. Lists staged, unstaged, and untracked files.",
		func(ctx context.Context, _ gitStatusRequest) (string, error) {
			return runGit(ctx, workdir, "status", "--short", "--branch")
		},
	)
}
