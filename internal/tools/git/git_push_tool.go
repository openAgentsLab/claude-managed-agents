package git

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type gitPushRequest struct {
	Remote string `json:"remote" jsonschema_description:"Remote name to push to (default: origin)"`
	Branch string `json:"branch" jsonschema_description:"Branch to push (default: current branch)"`
	Force  bool   `json:"force" jsonschema_description:"If true, force-push with --force"`
}

func newGitPushTool(workdir string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"git_push",
		"Push committed changes to a remote repository. Use git_commit first to create a commit.",
		func(ctx context.Context, in gitPushRequest) (string, error) {
			return handleGitPush(ctx, workdir, in)
		},
	)
}

func handleGitPush(ctx context.Context, workdir string, in gitPushRequest) (string, error) {
	remote := in.Remote
	if remote == "" {
		remote = "origin"
	}

	args := []string{"push"}
	if in.Force {
		args = append(args, "--force")
	}
	args = append(args, remote)
	if in.Branch != "" {
		args = append(args, in.Branch)
	}
	return runGit(ctx, workdir, args...)
}
