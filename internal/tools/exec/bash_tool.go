package exec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type bashRequest struct {
	Command         string `json:"command"           jsonschema_description:"Shell command to execute"`
	Timeout         int    `json:"timeout"           jsonschema_description:"Timeout in milliseconds, default 30000, max 120000"`
	Workdir         string `json:"workdir"           jsonschema_description:"Working directory relative to workspace root; defaults to workspace root"`
	Description     string `json:"description"       jsonschema_description:"Human-readable description of what the command does"`
	RunInBackground bool   `json:"run_in_background" jsonschema_description:"Run command in background and return immediately. Use task_id to stream output into the task system, or omit to write to a temp file."`
	TaskID          string `json:"task_id,omitempty" jsonschema_description:"Optional task ID. When set with run_in_background=true, stdout/stderr are streamed to the task's output file so TaskOutput can read progress in real time."`
}

// newBashTool creates the bash tool. taskOutputDir is the task store base
// directory used to resolve {taskId}.output paths; pass "" to disable task
// output integration (falls back to temp-file behaviour).
func newBashTool(workspaceRoot, taskOutputDir string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"bash",
		"Execute a shell command in the workspace. Returns stdout+stderr. Non-zero exit codes are returned as [exit_code N] suffix, not as errors.",
		func(ctx context.Context, in bashRequest) (string, error) {
			return handleBash(ctx, workspaceRoot, taskOutputDir, in)
		},
	)
}

func handleBash(ctx context.Context, workspaceRoot, taskOutputDir string, in bashRequest) (string, error) {
	if in.Command == "" {
		return "", nil
	}
	timeoutMs := normalizeTimeoutMs(in.Timeout)
	workdir, err := resolveWorkdir(workspaceRoot, in.Workdir)
	if err != nil {
		return "", err
	}
	argv := buildShellArgv(in.Command)

	if !in.RunInBackground {
		return runCommand(ctx, workdir, argv, timeoutMs)
	}

	// Determine output file: task-bound or anonymous temp file.
	// When task_id is provided, stdout/stderr are streamed (appended) to
	// {taskOutputDir}/{taskId}.output in real time so TaskOutput can read
	// progress before the command completes — mirroring TaskOutput.ts file
	// redirection in forge.
	var outPath string
	if in.TaskID != "" && taskOutputDir != "" {
		outPath = filepath.Join(taskOutputDir, in.TaskID+".output")
	} else {
		f, err := os.CreateTemp("", "bash-bg-*.txt")
		if err != nil {
			return "", fmt.Errorf("create background output file: %w", err)
		}
		outPath = f.Name()
		f.Close()
	}

	go func() {
		// Open (or create) the output file in append mode so partial output
		// is visible as soon as each chunk is written, mirroring the fd
		// redirection used by forge's TaskOutput.
		f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return
		}
		defer f.Close() //nolint:errcheck
		runCommandToWriter(context.Background(), workdir, argv, timeoutMs, f)
	}()
	return fmt.Sprintf("(running in background; output will be written to %s)", outPath), nil
}
