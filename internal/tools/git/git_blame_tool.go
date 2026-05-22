package git

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type gitBlameRequest struct {
	FilePath  string `json:"file_path" jsonschema_description:"File to blame (required)"`
	LineStart int    `json:"line_start" jsonschema_description:"First line of the range (1-based, optional)"`
	LineEnd   int    `json:"line_end" jsonschema_description:"Last line of the range (inclusive, optional)"`
}

func newGitBlameTool(workdir string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"git_blame",
		"Show per-line commit and author information for a file. Optionally restrict to a line range.",
		func(ctx context.Context, in gitBlameRequest) (string, error) {
			return handleGitBlame(ctx, workdir, in)
		},
	)
}

func handleGitBlame(ctx context.Context, workdir string, in gitBlameRequest) (string, error) {
	if in.FilePath == "" {
		return "", fmt.Errorf("file_path is required")
	}
	args := []string{"blame", "--date=short"}
	if in.LineStart > 0 && in.LineEnd >= in.LineStart {
		args = append(args, fmt.Sprintf("-L%d,%d", in.LineStart, in.LineEnd))
	}
	args = append(args, "--", in.FilePath)
	return runGit(ctx, workdir, args...)
}
