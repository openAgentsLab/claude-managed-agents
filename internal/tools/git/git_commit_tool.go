package git

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type gitCommitRequest struct {
	Message    string `json:"message" jsonschema_description:"Commit message (required)"`
	AllowEmpty bool   `json:"allow_empty" jsonschema_description:"Allow a commit with no staged changes"`
}

func newGitCommitTool(workdir string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"git_commit",
		"Create a commit with the staged changes. Use git_add first to stage files.",
		func(ctx context.Context, in gitCommitRequest) (string, error) {
			return handleGitCommit(ctx, workdir, in)
		},
	)
}

func handleGitCommit(ctx context.Context, workdir string, in gitCommitRequest) (string, error) {
	if in.Message == "" {
		return "", fmt.Errorf("commit message is required")
	}
	args := []string{"commit", "-m", in.Message}
	if in.AllowEmpty {
		args = append(args, "--allow-empty")
	}
	return runGit(ctx, workdir, args...)
}
