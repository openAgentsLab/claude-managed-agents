package fs

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type deleteFileRequest struct {
	FilePath string `json:"file_path" jsonschema_description:"Absolute file path under workspace to delete"`
}

func newDeleteFileTool(root string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"delete_file",
		"Delete a single file. Returns an error if the path is a directory or does not exist.",
		func(ctx context.Context, in deleteFileRequest) (string, error) {
			return handleDeleteFile(root, in)
		},
	)
}

func handleDeleteFile(root string, in deleteFileRequest) (string, error) {
	full, err := safeJoinAbsolute(root, in.FilePath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory; use delete_dir instead", in.FilePath)
	}

	if err := os.Remove(full); err != nil {
		return "", err
	}
	return fmt.Sprintf("Deleted %s", in.FilePath), nil
}
