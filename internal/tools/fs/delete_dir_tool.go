package fs

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type deleteDirRequest struct {
	DirPath   string `json:"dir_path" jsonschema_description:"Absolute directory path under workspace to delete"`
	Recursive bool   `json:"recursive" jsonschema_description:"If true, delete the directory and all its contents; if false, only delete if the directory is empty"`
}

func newDeleteDirTool(root string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"delete_dir",
		"Delete a directory. Set recursive=true to delete non-empty directories. Without recursive=true the call fails if the directory is not empty.",
		func(ctx context.Context, in deleteDirRequest) (string, error) {
			return handleDeleteDir(root, in)
		},
	)
}

func handleDeleteDir(root string, in deleteDirRequest) (string, error) {
	full, err := safeJoinAbsolute(root, in.DirPath)
	if err != nil {
		return "", err
	}

	// Disallow deleting the workspace root itself.
	if full == root {
		return "", fmt.Errorf("cannot delete workspace root")
	}

	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory; use delete_file instead", in.DirPath)
	}

	if in.Recursive {
		if err := os.RemoveAll(full); err != nil {
			return "", err
		}
	} else {
		// os.Remove on a non-empty directory returns a "directory not empty" error.
		if err := os.Remove(full); err != nil {
			return "", fmt.Errorf("%s: %w (set recursive=true to delete non-empty directories)", in.DirPath, err)
		}
	}
	return fmt.Sprintf("Deleted directory %s", in.DirPath), nil
}
