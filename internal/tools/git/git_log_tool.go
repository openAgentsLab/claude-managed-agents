package git

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type gitLogRequest struct {
	Limit    int    `json:"limit" jsonschema_description:"Maximum number of commits to show (default 20)"`
	Branch   string `json:"branch" jsonschema_description:"Branch or ref to log (default: current branch)"`
	FilePath string `json:"file_path" jsonschema_description:"Limit history to commits touching this file (optional)"`
}

func newGitLogTool(workdir string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"git_log",
		"Show commit history in a compact one-line format with hash, author, date, and subject.",
		func(ctx context.Context, in gitLogRequest) (string, error) {
			return handleGitLog(ctx, workdir, in)
		},
	)
}

func handleGitLog(ctx context.Context, workdir string, in gitLogRequest) (string, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}

	args := []string{
		"log",
		fmt.Sprintf("--max-count=%d", limit),
		"--pretty=format:%h  %ad  %an  %s",
		"--date=short",
	}

	if in.Branch != "" {
		args = append(args, in.Branch)
	}
	if in.FilePath != "" {
		args = append(args, "--", in.FilePath)
	}
	return runGit(ctx, workdir, args...)
}
