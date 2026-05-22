package git

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type gitCheckoutRequest struct {
	Branch string `json:"branch" jsonschema_description:"Branch name to switch to or create (required)"`
	Create bool   `json:"create" jsonschema_description:"If true, create the branch before switching (-b)"`
}

func newGitCheckoutTool(workdir string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"git_checkout",
		"Switch to a branch, or create and switch to a new one with create=true. Returns an error if there are uncommitted changes that would be overwritten.",
		func(ctx context.Context, in gitCheckoutRequest) (string, error) {
			return handleGitCheckout(ctx, workdir, in)
		},
	)
}

func handleGitCheckout(ctx context.Context, workdir string, in gitCheckoutRequest) (string, error) {
	if in.Branch == "" {
		return "", fmt.Errorf("branch is required")
	}
	args := []string{"checkout"}
	if in.Create {
		args = append(args, "-b")
	}
	args = append(args, in.Branch)
	// git itself will error with a clear message if the working tree is dirty
	// and the switch would overwrite local changes, so no pre-flight needed.
	return runGit(ctx, workdir, args...)
}
