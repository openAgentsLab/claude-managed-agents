package fs

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type globRequest struct {
	Pattern string `json:"pattern" jsonschema_description:"Glob pattern, e.g. **/*.go or src/**/*.ts"`
	Path    string `json:"path" jsonschema_description:"Root directory to search; defaults to workspace root"`
}

const globMaxResults = 500

func newGlobTool(root string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"glob",
		"Find files matching a glob pattern. Returns matching paths sorted by modification time (newest first), up to 500 results.",
		func(ctx context.Context, in globRequest) (string, error) {
			return handleGlob(root, in)
		},
	)
}

type globEntry struct {
	rel   string
	mtime time.Time
}

func handleGlob(root string, in globRequest) (string, error) {
	pattern := strings.TrimSpace(in.Pattern)
	if pattern == "" {
		return "", fmt.Errorf("pattern required")
	}

	searchRoot := root
	if strings.TrimSpace(in.Path) != "" {
		var err error
		searchRoot, err = safeJoinAny(root, in.Path)
		if err != nil {
			return "", err
		}
	}

	// doublestar.FS requires an fs.FS; use os.DirFS.
	fsys := os.DirFS(searchRoot)

	var entries []globEntry
	capped := false

	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		// Skip hidden directories and node_modules.
		if d.IsDir() && path != "." {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" {
				return fs.SkipDir
			}
		}
		if d.IsDir() {
			return nil
		}

		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			return nil // invalid pattern segment — skip
		}
		if !matched {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		entries = append(entries, globEntry{rel: filepath.ToSlash(path), mtime: info.ModTime()})
		if len(entries) >= globMaxResults {
			capped = true
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("glob walk: %w", err)
	}

	if len(entries) == 0 {
		return "No files found.", nil
	}

	// Sort newest-first by modification time.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].mtime.After(entries[j].mtime)
	})

	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = e.rel
	}
	result := strings.Join(lines, "\n")
	if capped {
		result += fmt.Sprintf("\n... (results capped at %d)", globMaxResults)
	}
	return result, nil
}
