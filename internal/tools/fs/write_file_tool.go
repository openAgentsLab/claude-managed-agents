package fs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type writeFileRequest struct {
	FilePath string `json:"file_path" jsonschema_description:"Absolute file path under workspace"`
	Content  string `json:"content" jsonschema_description:"Full content to write; existing file is overwritten"`
}

func newWriteFileTool(root string, activator SkillActivator, discoverer SkillDiscoverer) (tool.InvokableTool, error) {
	return utils.InferTool(
		"write_file",
		"Write (overwrite) a file with the given content. Parent directories are created automatically. Prefer file_edit for partial changes.",
		func(ctx context.Context, in writeFileRequest) (string, error) {
			r := root
			result, err := handleWriteFile(r, in)
			if err == nil {
				full, _ := safeJoinAbsolute(r, in.FilePath)
				if notification := notifySkillHooks(full, r, discoverer, activator); notification != "" {
					result += "\n" + notification
				}
			}
			return result, err
		},
	)
}

func handleWriteFile(root string, in writeFileRequest) (string, error) {
	full, err := safeJoinAbsolute(root, in.FilePath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return "", fmt.Errorf("create parent directories: %w", err)
	}
	if err := writeFile(full, in.Content); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wrote %s (%d bytes)", in.FilePath, len(in.Content)), nil
}
