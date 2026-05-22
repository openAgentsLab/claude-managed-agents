package fs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type fileEditRequest struct {
	FilePath   string `json:"file_path" jsonschema_description:"Absolute file path under workspace"`
	OldString  string `json:"old_string" jsonschema_description:"Exact string to replace; empty string creates a new file"`
	NewString  string `json:"new_string" jsonschema_description:"Replacement string"`
	ReplaceAll bool   `json:"replace_all" jsonschema_description:"Replace all occurrences; required when old_string appears more than once"`
}

func newFileEditTool(root string, activator SkillActivator, discoverer SkillDiscoverer) (tool.InvokableTool, error) {
	return utils.InferTool(
		"file_edit",
		"Edit a file by replacing an exact string. old_string must be unique in the file unless replace_all=true. Pass empty old_string to create a new file.",
		func(ctx context.Context, in fileEditRequest) (string, error) {
			r := root
			result, err := handleFileEdit(r, in)
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

func handleFileEdit(root string, in fileEditRequest) (string, error) {
	full, err := safeJoinAbsolute(root, in.FilePath)
	if err != nil {
		return "", err
	}

	// Create new file when old_string is empty and file does not exist.
	if in.OldString == "" {
		if _, err := os.Stat(full); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
				return "", fmt.Errorf("create parent directories: %w", err)
			}
			if err := writeFile(full, in.NewString); err != nil {
				return "", err
			}
			return fmt.Sprintf("Created %s", in.FilePath), nil
		}
		return "", fmt.Errorf("file already exists; provide old_string to edit it")
	}

	if in.OldString == in.NewString {
		return "", fmt.Errorf("old_string and new_string are identical; no edit needed")
	}

	content, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	text := string(content)

	count := strings.Count(text, in.OldString)
	if count == 0 {
		return "", fmt.Errorf("old_string not found in %s", in.FilePath)
	}
	if count > 1 && !in.ReplaceAll {
		return "", fmt.Errorf("old_string appears %d times; set replace_all=true or add more context to make it unique", count)
	}

	var updated string
	if in.ReplaceAll {
		updated = strings.ReplaceAll(text, in.OldString, in.NewString)
	} else {
		updated = strings.Replace(text, in.OldString, in.NewString, 1)
	}

	if err := writeFile(full, updated); err != nil {
		return "", err
	}

	replaced := 1
	if in.ReplaceAll {
		replaced = count
	}

	diff := unifiedDiff(in.OldString, in.NewString)
	return fmt.Sprintf("Edited %s (%d replacement(s))\n%s", in.FilePath, replaced, diff), nil
}

// unifiedDiff returns a compact unified-style diff of old vs new.
func unifiedDiff(old, new string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(old, new, true)
	dmp.DiffCleanupSemantic(diffs)

	var sb strings.Builder
	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		switch d.Type {
		case diffmatchpatch.DiffDelete:
			for _, l := range lines {
				if l != "" {
					sb.WriteString("- ")
					sb.WriteString(l)
					sb.WriteByte('\n')
				}
			}
		case diffmatchpatch.DiffInsert:
			for _, l := range lines {
				if l != "" {
					sb.WriteString("+ ")
					sb.WriteString(l)
					sb.WriteByte('\n')
				}
			}
		}
	}
	return sb.String()
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
