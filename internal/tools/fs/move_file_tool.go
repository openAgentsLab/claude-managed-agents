package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type moveFileRequest struct {
	SrcPath string `json:"src_path" jsonschema_description:"Absolute source path under workspace"`
	DstPath string `json:"dst_path" jsonschema_description:"Absolute destination path under workspace; parent directories are created automatically"`
}

func newMoveFileTool(root string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"move_file",
		"Move or rename a file or directory. Works across devices by falling back to copy+delete when needed.",
		func(ctx context.Context, in moveFileRequest) (string, error) {
			return handleMoveFile(root, in)
		},
	)
}

func handleMoveFile(root string, in moveFileRequest) (string, error) {
	src, err := safeJoinAbsolute(root, in.SrcPath)
	if err != nil {
		return "", fmt.Errorf("src_path: %w", err)
	}
	dst, err := safeJoinAbsolute(root, in.DstPath)
	if err != nil {
		return "", fmt.Errorf("dst_path: %w", err)
	}

	if src == dst {
		return "", fmt.Errorf("source and destination are the same path")
	}

	if _, err := os.Stat(src); err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", fmt.Errorf("create destination parent dirs: %w", err)
	}

	// Fast path: same device rename.
	if err := os.Rename(src, dst); err == nil {
		return fmt.Sprintf("Moved %s → %s", in.SrcPath, in.DstPath), nil
	}

	// Cross-device fallback: copy then delete.
	if err := copyFile(src, dst); err != nil {
		return "", fmt.Errorf("cross-device copy failed: %w", err)
	}
	if err := os.Remove(src); err != nil {
		// dst was already written; attempt cleanup but surface original error.
		_ = os.Remove(dst)
		return "", fmt.Errorf("remove source after copy: %w", err)
	}
	return fmt.Sprintf("Moved %s → %s (cross-device)", in.SrcPath, in.DstPath), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
