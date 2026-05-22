package git

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type gitDiffRequest struct {
	Staged   bool   `json:"staged" jsonschema_description:"If true, show staged (cached) changes; otherwise show unstaged working-tree changes"`
	FilePath string `json:"file_path" jsonschema_description:"Limit diff to this file path (optional)"`
}

func newGitDiffTool(workdir string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"git_diff",
		"Show diff of working tree or staged changes. Exit code 1 (meaning 'has differences') is treated as success.",
		func(ctx context.Context, in gitDiffRequest) (string, error) {
			return handleGitDiff(ctx, workdir, in)
		},
	)
}

func handleGitDiff(ctx context.Context, workdir string, in gitDiffRequest) (string, error) {
	args := []string{"diff"}
	if in.Staged {
		args = append(args, "--staged")
	}
	if in.FilePath != "" {
		args = append(args, "--", in.FilePath)
	}
	// exit 1 = "has differences" is normal; use permissive runner.
	return runGitPermissive(ctx, workdir, args...)
}
