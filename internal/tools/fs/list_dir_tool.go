package fs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type listDirRequest struct {
	DirPath string `json:"dir_path" jsonschema_description:"absolute directory path under workspace"`
	Offset  int    `json:"offset" jsonschema_description:"1-indexed entry number, default 1"`
	Limit   int    `json:"limit" jsonschema_description:"maximum entries, default 25"`
	Depth   int    `json:"depth" jsonschema_description:"directory depth, default 2"`
}

const (
	listDefaultOffset = 1
	listDefaultLimit  = 25
	listDefaultDepth  = 2
)

func newListDirTool(root string) (tool.InvokableTool, error) {
	return utils.InferTool("list_dir", "List directory entries with depth/offset/limit. Codex-style args: dir_path, offset, limit, depth.", func(ctx context.Context, in listDirRequest) (string, error) {
		return handleListDir(root, in)
	})
}

func handleListDir(root string, in listDirRequest) (string, error) {
	offset, limit, depth := normalizeListDirArgs(in)
	full, err := resolveListDirPath(root, in.DirPath)
	if err != nil {
		return "", err
	}
	entries, err := loadListDirEntries(full, depth)
	if err != nil {
		return "", err
	}
	return formatListDirEntries(full, entries, offset, limit)
}

func resolveListDirPath(root, dirPath string) (string, error) {
	full, err := safeJoinAbsolute(root, dirPath)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("path is not directory: %s", dirPath)
	}
	return full, nil
}

func loadListDirEntries(full string, depth int) ([]dirEntry, error) {
	entries, err := collectDirEntries(full, depth)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func formatListDirEntries(full string, entries []dirEntry, offset, limit int) (string, error) {
	start := offset - 1
	if start >= len(entries) {
		return "", fmt.Errorf("offset exceeds directory entry count")
	}
	end := start + limit
	if end > len(entries) {
		end = len(entries)
	}

	var out strings.Builder
	out.WriteString("Absolute path: " + full + "\n")
	for _, e := range entries[start:end] {
		out.WriteString(formatDirEntryLine(e))
		out.WriteByte('\n')
	}
	if end < len(entries) {
		out.WriteString(fmt.Sprintf("More than %d entries found\n", end-start))
	}
	return out.String(), nil
}

func normalizeListDirArgs(in listDirRequest) (offset int, limit int, depth int) {
	offset = in.Offset
	if offset <= 0 {
		offset = listDefaultOffset
	}
	limit = in.Limit
	if limit <= 0 {
		limit = listDefaultLimit
	}
	depth = in.Depth
	if depth <= 0 {
		depth = listDefaultDepth
	}
	return offset, limit, depth
}

type dirEntryKind int

const (
	dirFile dirEntryKind = iota
	dirDirectory
	dirSymlink
	dirOther
)

type dirEntry struct {
	Name        string
	DisplayName string
	Depth       int
	Kind        dirEntryKind
}

func collectDirEntries(base string, maxDepth int) ([]dirEntry, error) {
	type item struct {
		abs   string
		rel   string
		depth int
	}
	queue := []item{{abs: base, rel: "", depth: maxDepth}}
	out := make([]dirEntry, 0, 64)
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		ents, err := os.ReadDir(it.abs)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}
		sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })
		for _, e := range ents {
			absPath := filepath.Join(it.abs, e.Name())
			relPath := e.Name()
			if it.rel != "" {
				relPath = filepath.Join(it.rel, e.Name())
			}
			kind := classifyDirEntry(absPath, e)
			out = append(out, dirEntry{
				Name:        filepath.ToSlash(relPath),
				DisplayName: e.Name(),
				Depth:       strings.Count(filepath.ToSlash(relPath), "/"),
				Kind:        kind,
			})
			if kind == dirDirectory && it.depth > 1 {
				queue = append(queue, item{abs: absPath, rel: relPath, depth: it.depth - 1})
			}
		}
	}
	return out, nil
}

func classifyDirEntry(absPath string, e os.DirEntry) dirEntryKind {
	if e.Type()&os.ModeSymlink != 0 {
		return dirSymlink
	}
	if e.IsDir() {
		return dirDirectory
	}
	if e.Type().IsRegular() {
		return dirFile
	}
	if st, err := os.Stat(absPath); err == nil {
		if st.IsDir() {
			return dirDirectory
		}
		if st.Mode().IsRegular() {
			return dirFile
		}
	}
	return dirOther
}

func formatDirEntryLine(e dirEntry) string {
	indent := strings.Repeat(" ", e.Depth*2)
	name := e.DisplayName
	switch e.Kind {
	case dirDirectory:
		name += "/"
	case dirSymlink:
		name += "@"
	case dirOther:
		name += "?"
	}
	return indent + name
}
