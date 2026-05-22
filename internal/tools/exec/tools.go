package exec

import "github.com/cloudwego/eino/components/tool"

// NewExecTools returns command execution tools.
// taskOutputDir is the task store base directory (e.g. ~/.forge/tasks/{listID});
// pass "" to disable task output integration for background commands.
func NewExecTools(workspaceRoot, taskOutputDir string) ([]tool.BaseTool, error) {
	bashTool, err := newBashTool(workspaceRoot, taskOutputDir)
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{bashTool}, nil
}
