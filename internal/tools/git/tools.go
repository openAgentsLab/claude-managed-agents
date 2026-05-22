package git

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
)

// NewGitTools returns all git tools. If git is not installed, returns an empty
// slice with a warning log rather than failing startup.
func NewGitTools(workspaceRoot string) ([]tool.BaseTool, error) {
	if _, err := exec.LookPath("git"); err != nil {
		log.Printf("[warn] git not found, skipping git tools: %v", err)
		return nil, nil
	}

	workdir, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}

	status, err := newGitStatusTool(workdir)
	if err != nil {
		return nil, err
	}
	diff, err := newGitDiffTool(workdir)
	if err != nil {
		return nil, err
	}
	logTool, err := newGitLogTool(workdir)
	if err != nil {
		return nil, err
	}
	blame, err := newGitBlameTool(workdir)
	if err != nil {
		return nil, err
	}
	add, err := newGitAddTool(workdir)
	if err != nil {
		return nil, err
	}
	commit, err := newGitCommitTool(workdir)
	if err != nil {
		return nil, err
	}
	checkout, err := newGitCheckoutTool(workdir)
	if err != nil {
		return nil, err
	}
	show, err := newGitShowTool(workdir)
	if err != nil {
		return nil, err
	}
	push, err := newGitPushTool(workdir)
	if err != nil {
		return nil, err
	}

	return []tool.BaseTool{status, diff, logTool, blame, add, commit, checkout, show, push}, nil
}
