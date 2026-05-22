package git

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type gitShowRequest struct {
	Ref      string `json:"ref" jsonschema_description:"Commit hash, tag, or branch to inspect (required)"`
	FilePath string `json:"file_path" jsonschema_description:"If set, show only this file's content at the given ref"`
}

func newGitShowTool(workdir string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"git_show",
		"Show a commit's metadata and diff, or the content of a specific file at a given ref.",
		func(ctx context.Context, in gitShowRequest) (string, error) {
			return handleGitShow(ctx, workdir, in)
		},
	)
}

func handleGitShow(ctx context.Context, workdir string, in gitShowRequest) (string, error) {
	if in.Ref == "" {
		return "", fmt.Errorf("ref is required")
	}
	if in.FilePath != "" {
		// Show file content at ref: git show <ref>:<file>
		return runGit(ctx, workdir, "show", fmt.Sprintf("%s:%s", in.Ref, in.FilePath))
	}
	return runGit(ctx, workdir, "show", "--stat", in.Ref)
}
