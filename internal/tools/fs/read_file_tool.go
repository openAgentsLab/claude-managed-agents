package fs

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type readFileRequest struct {
	FilePath string `json:"file_path" jsonschema_description:"absolute file path under workspace"`
	Offset   int    `json:"offset" jsonschema_description:"1-based start line, default 1"`
	Limit    int    `json:"limit" jsonschema_description:"maximum lines, default 2000"`
}

const (
	readDefaultOffset = 1
	readDefaultLimit  = 2000
)

func newReadFileTool(root string, activator SkillActivator, discoverer SkillDiscoverer) (tool.InvokableTool, error) {
	return utils.InferTool("read_file", "Read file content with line numbers. Codex-style args: file_path, offset, limit, mode.", func(ctx context.Context, in readFileRequest) (string, error) {
		r := root
		result, err := handleReadFile(r, in)
		if err == nil {
			full, _ := safeJoinAbsolute(r, in.FilePath)
			if notification := notifySkillHooks(full, r, discoverer, activator); notification != "" {
				result += "\n" + notification
			}
		}
		return result, err
	})
}

func handleReadFile(root string, in readFileRequest) (string, error) {
	offset, limit, err := normalizeReadFileArgs(in)
	if err != nil {
		return "", err
	}

	full, err := resolveReadFilePath(root, in.FilePath)
	if err != nil {
		return "", err
	}
	lines, err := loadReadFileLines(full)
	if err != nil {
		return "", err
	}
	return formatReadFileLines(lines, offset, limit)
}

func resolveReadFilePath(root, filePath string) (string, error) {
	full, err := safeJoinAbsolute(root, filePath)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("path is directory: %s", filePath)
	}
	return full, nil
}

func loadReadFileLines(fullPath string) ([]string, error) {
	b, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(b), "\n"), nil
}

func formatReadFileLines(lines []string, offset, limit int) (string, error) {
	start := offset - 1
	if start >= len(lines) {
		return "", fmt.Errorf("offset exceeds file length")
	}

	var out strings.Builder
	count := 0
	for i := start; i < len(lines) && count < limit; i++ {
		fmt.Fprintf(&out, "L%d: %s\n", i+1, lines[i])
		count++
	}
	return out.String(), nil
}

func normalizeReadFileArgs(in readFileRequest) (offset int, limit int, err error) {
	offset = in.Offset
	if offset <= 0 {
		offset = readDefaultOffset
	}
	limit = in.Limit
	if limit <= 0 {
		limit = readDefaultLimit
	}
	return offset, limit, nil
}
