package fs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type grepFileRequest struct {
	Pattern         string `json:"pattern" jsonschema_description:"Regular expression pattern to search for"`
	Path            string `json:"path" jsonschema_description:"Absolute or relative path under workspace, default workspace root"`
	Glob            string `json:"glob" jsonschema_description:"Optional file glob filter, e.g. *.go or **/*.ts"`
	OutputMode      string `json:"output_mode" jsonschema_description:"Output mode: files_with_matches (default), content (matching lines with context), or count"`
	HeadLimit       int    `json:"head_limit" jsonschema_description:"Max results to return, default 250"`
	CaseInsensitive bool   `json:"case_insensitive" jsonschema_description:"Case-insensitive match"`
	Context         int    `json:"context" jsonschema_description:"Lines of context before and after each match (content mode only); overridden by -A/-B when both set"`
	After           int    `json:"after" jsonschema_description:"Lines of context after each match (content mode only)"`
	Before          int    `json:"before" jsonschema_description:"Lines of context before each match (content mode only)"`
}

const (
	grepDefaultHeadLimit = 250
	grepMaxHeadLimit     = 2000
)

func newGrepFileTool(root string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"grep",
		"Search file contents using ripgrep. Supports content/files_with_matches/count output modes.",
		func(ctx context.Context, in grepFileRequest) (string, error) {
			return handleGrepFile(ctx, root, in)
		},
	)
}

func handleGrepFile(ctx context.Context, root string, in grepFileRequest) (string, error) {
	if strings.TrimSpace(in.Pattern) == "" {
		return "", fmt.Errorf("pattern required")
	}

	searchPath := root
	if strings.TrimSpace(in.Path) != "" {
		var err error
		searchPath, err = safeJoinAny(root, in.Path)
		if err != nil {
			return "", err
		}
	}

	mode := strings.TrimSpace(strings.ToLower(in.OutputMode))
	if mode == "" {
		mode = "files_with_matches"
	}
	switch mode {
	case "files_with_matches", "content", "count":
	default:
		return "", fmt.Errorf("output_mode must be files_with_matches, content, or count")
	}

	limit := in.HeadLimit
	if limit <= 0 {
		limit = grepDefaultHeadLimit
	}
	if limit > grepMaxHeadLimit {
		limit = grepMaxHeadLimit
	}

	args := buildGrepArgs(in, searchPath, mode)
	out, err := runGrepCmd(ctx, root, args)
	if err != nil {
		return "", err
	}
	return formatGrepOutput(out, mode, limit), nil
}

func buildGrepArgs(in grepFileRequest, searchPath, mode string) []string {
	args := []string{"--no-messages", "--regexp", in.Pattern}

	switch mode {
	case "files_with_matches":
		args = append(args, "--files-with-matches", "--sortr=modified")
	case "count":
		args = append(args, "--count", "--sort-files")
	case "content":
		args = append(args, "--line-number", "--sortr=modified")
		// -A and -B take precedence over -C when specified.
		if in.After > 0 || in.Before > 0 {
			if in.After > 0 {
				args = append(args, fmt.Sprintf("--after-context=%d", in.After))
			}
			if in.Before > 0 {
				args = append(args, fmt.Sprintf("--before-context=%d", in.Before))
			}
		} else if in.Context > 0 {
			args = append(args, fmt.Sprintf("--context=%d", in.Context))
		}
	}

	if in.CaseInsensitive {
		args = append(args, "--ignore-case")
	}
	if in.Glob != "" {
		args = append(args, "--glob", in.Glob)
	}
	// Exclude common VCS dirs.
	args = append(args, "--glob", "!.git", "--glob", "!.svn", "--glob", "!.hg")
	args = append(args, "--", searchPath)
	return args
}

func runGrepCmd(ctx context.Context, root string, args []string) ([]byte, error) {
	ctxT, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctxT, "rg", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil, nil // exit 1 = no matches
		}
		return nil, fmt.Errorf("rg failed: %w", err)
	}
	return out, nil
}

func formatGrepOutput(out []byte, mode string, limit int) string {
	if len(out) == 0 {
		return "No matches found."
	}
	text := strings.TrimSpace(string(out))

	// In content mode rg uses "--" separators between files; keep them intact but still cap line count.
	lines := strings.Split(text, "\n")
	var kept []string
	for _, l := range lines {
		kept = append(kept, l)
		if mode != "content" && len(kept) >= limit {
			break
		}
		if mode == "content" && countNonSeparatorLines(kept) >= limit {
			break
		}
	}

	result := strings.Join(kept, "\n")
	if len(kept) < len(lines) {
		result += fmt.Sprintf("\n... (%d more lines truncated)", len(lines)-len(kept))
	}
	return result
}

func countNonSeparatorLines(lines []string) int {
	n := 0
	for _, l := range lines {
		if l != "--" {
			n++
		}
	}
	return n
}
