package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type gitAddRequest struct {
	Paths []string `json:"paths" jsonschema_description:"List of file paths to stage. Use [\".\"] to stage all changes."`
}

func newGitAddTool(workdir string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"git_add",
		"Stage files for commit. Paths are validated to stay within the workspace.",
		func(ctx context.Context, in gitAddRequest) (string, error) {
			return handleGitAdd(ctx, workdir, in)
		},
	)
}

func handleGitAdd(ctx context.Context, workdir string, in gitAddRequest) (string, error) {
	if len(in.Paths) == 0 {
		return "", fmt.Errorf("paths must not be empty")
	}

	// Validate and normalise each path.
	safe := make([]string, 0, len(in.Paths))
	for _, p := range in.Paths {
		if p == "." {
			safe = append(safe, ".")
			continue
		}
		full := filepath.Clean(p)
		if !filepath.IsAbs(full) {
			full = filepath.Join(workdir, full)
		}
		full = filepath.Clean(full)
		// Ensure the path stays within the workspace.
		if full != workdir && !strings.HasPrefix(full, workdir+string(filepath.Separator)) {
			return "", fmt.Errorf("path escapes workspace: %s", p)
		}
		// Pass relative path to git (cwd is workspace root).
		rel, err := filepath.Rel(workdir, full)
		if err != nil {
			return "", err
		}
		safe = append(safe, rel)
	}

	args := append([]string{"add", "--"}, safe...)
	return runGit(ctx, workdir, args...)
}
