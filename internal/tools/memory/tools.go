package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"forge/internal/memory"
)

// NewTools returns the 6 memory tools.
// They obtain SessionStores from the context at call time.
func NewTools() ([]tool.BaseTool, error) {
	listTool, err := utils.InferTool(
		"memory_list",
		"List all documents across every mounted memory store. Returns a grouped index with store descriptions and document excerpts. Call this at the start of a task to recall relevant context.",
		func(ctx context.Context, _ struct{}) (string, error) {
			ss := memory.SessionStoresFromContext(ctx)
			if ss == nil {
				return "(no memory stores mounted)", nil
			}
			result := ss.MergeList()
			if result == "" {
				return "(all stores are empty)", nil
			}
			return result, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("memory tools: memory_list: %w", err)
	}

	readTool, err := utils.InferTool(
		"memory_read",
		"Read the full content of a memory document. Returns content, store name, and current sha256 for optimistic concurrency.",
		func(ctx context.Context, req struct {
			Filename string `json:"filename" jsonschema_description:"Document filename (e.g. preferences.md)"`
			Store    string `json:"store,omitempty" jsonschema_description:"Store name to read from. If omitted, searches all mounted stores."`
		}) (string, error) {
			ss := memory.SessionStoresFromContext(ctx)
			if ss == nil {
				return "", fmt.Errorf("no memory stores mounted")
			}
			if req.Store != "" {
				st, found, _ := ss.FindStore(req.Store)
				if !found {
					return "", fmt.Errorf("store %q not found", req.Store)
				}
				doc, found, err := st.Read(req.Filename)
				if err != nil {
					return "", err
				}
				if !found {
					return "", fmt.Errorf("document %q not found in store %q", req.Filename, req.Store)
				}
				return formatReadResult(doc, st.Name()), nil
			}
			// Search all stores; collect errors so they are visible to the caller.
			var storeErrs []string
			for _, m := range ss.MountedStores() {
				doc, found, err := m.Store.Read(req.Filename)
				if err != nil {
					storeErrs = append(storeErrs, m.Store.Name()+": "+err.Error())
					continue
				}
				if found {
					return formatReadResult(doc, m.Store.Name()), nil
				}
			}
			if len(storeErrs) > 0 {
				return "", fmt.Errorf("document %q not found; store errors: %s",
					req.Filename, strings.Join(storeErrs, "; "))
			}
			return "", fmt.Errorf("document %q not found in any mounted store", req.Filename)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("memory tools: memory_read: %w", err)
	}

	searchTool, err := utils.InferTool(
		"memory_search",
		"Full-text search across memory stores. Returns up to 5 matching document excerpts per store.",
		func(ctx context.Context, req struct {
			Query string `json:"query" jsonschema_description:"Search query string"`
			Store string `json:"store,omitempty" jsonschema_description:"Limit search to this store. If omitted, searches all mounted stores."`
		}) (string, error) {
			ss := memory.SessionStoresFromContext(ctx)
			if ss == nil {
				return "(no memory stores mounted)", nil
			}
			var results []memory.SearchResult
			if req.Store != "" {
				st, found, _ := ss.FindStore(req.Store)
				if !found {
					return "", fmt.Errorf("store %q not found", req.Store)
				}
				r, err := st.Search(req.Query)
				if err != nil {
					return "", err
				}
				results = r
			} else {
				var storeErrs []string
				for _, m := range ss.MountedStores() {
					r, err := m.Store.Search(req.Query)
					if err != nil {
						storeErrs = append(storeErrs, m.Store.Name()+": "+err.Error())
						continue
					}
					results = append(results, r...)
				}
				if len(results) == 0 {
					if len(storeErrs) > 0 {
						return fmt.Sprintf("No results found. Store errors: %s", strings.Join(storeErrs, "; ")), nil
					}
					return "No results found.", nil
				}
				var sb strings.Builder
				for _, r := range results {
					sb.WriteString(fmt.Sprintf("[%s/%s] %s\n", r.StoreName, r.Filename, r.Excerpt))
				}
				if len(storeErrs) > 0 {
					sb.WriteString("(search failed in: " + strings.Join(storeErrs, ", ") + ")")
				}
				return strings.TrimRight(sb.String(), "\n"), nil
			}
			if len(results) == 0 {
				return "No results found.", nil
			}
			var sb strings.Builder
			for _, r := range results {
				sb.WriteString(fmt.Sprintf("[%s/%s] %s\n", r.StoreName, r.Filename, r.Excerpt))
			}
			return strings.TrimRight(sb.String(), "\n"), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("memory tools: memory_search: %w", err)
	}

	writeTool, err := utils.InferTool(
		"memory_write",
		"Create or fully replace a memory document (upsert-replace, not append). Returns the new sha256 for subsequent optimistic concurrency checks.",
		func(ctx context.Context, req struct {
			Filename string `json:"filename" jsonschema_description:"Document filename (must match ^[a-z0-9_\\-]+\\.md$)"`
			Content  string `json:"content" jsonschema_description:"Full document content (Markdown recommended)"`
			Store    string `json:"store" jsonschema_description:"Target store name (required)"`
			SHA256   string `json:"sha256,omitempty" jsonschema_description:"Current content sha256 for optimistic concurrency. Omit to force-overwrite."`
		}) (string, error) {
			ss := memory.SessionStoresFromContext(ctx)
			if ss == nil {
				return "", fmt.Errorf("no memory stores mounted")
			}
			if req.Store == "" {
				return "", fmt.Errorf("store parameter is required")
			}
			if strings.TrimSpace(req.Content) == "" {
				return "", fmt.Errorf("content must not be empty")
			}
			st, found, writable := ss.FindStore(req.Store)
			if !found {
				return "", fmt.Errorf("store %q not found", req.Store)
			}
			if !writable {
				return "", fmt.Errorf("store %q is read-only in this session", req.Store)
			}
			newSHA, err := st.Write(req.Filename, req.Content, req.SHA256)
			if err != nil {
				var ce *memory.ConflictError
				if errors.As(err, &ce) {
					return "", fmt.Errorf("conflict: document was modified by another session (current sha256: %s)", ce.CurrentSHA256)
				}
				return "", err
			}
			return fmt.Sprintf("Written %s to store %q. New sha256: %s", req.Filename, req.Store, newSHA), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("memory tools: memory_write: %w", err)
	}

	editTool, err := utils.InferTool(
		"memory_edit",
		"Apply a str_replace edit to a memory document. old_str must appear exactly once in the document. Returns the new sha256.",
		func(ctx context.Context, req struct {
			Filename string `json:"filename" jsonschema_description:"Document filename"`
			OldStr   string `json:"old_str" jsonschema_description:"Exact string to replace (must appear exactly once)"`
			NewStr   string `json:"new_str" jsonschema_description:"Replacement string"`
			Store    string `json:"store" jsonschema_description:"Target store name (required)"`
			SHA256   string `json:"sha256,omitempty" jsonschema_description:"Current content sha256 for optimistic concurrency. Omit to skip check."`
		}) (string, error) {
			ss := memory.SessionStoresFromContext(ctx)
			if ss == nil {
				return "", fmt.Errorf("no memory stores mounted")
			}
			if req.Store == "" {
				return "", fmt.Errorf("store parameter is required")
			}
			st, found, writable := ss.FindStore(req.Store)
			if !found {
				return "", fmt.Errorf("store %q not found", req.Store)
			}
			if !writable {
				return "", fmt.Errorf("store %q is read-only in this session", req.Store)
			}
			newSHA, err := st.Edit(req.Filename, req.OldStr, req.NewStr, req.SHA256)
			if err != nil {
				var ce *memory.ConflictError
				if errors.As(err, &ce) {
					return "", fmt.Errorf("conflict: document was modified by another session (current sha256: %s)", ce.CurrentSHA256)
				}
				return "", err
			}
			return fmt.Sprintf("Edited %s in store %q. New sha256: %s", req.Filename, req.Store, newSHA), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("memory tools: memory_edit: %w", err)
	}

	deleteTool, err := utils.InferTool(
		"memory_delete",
		"Delete a memory document. Silently succeeds when the document does not exist.",
		func(ctx context.Context, req struct {
			Filename string `json:"filename" jsonschema_description:"Document filename to delete"`
			Store    string `json:"store" jsonschema_description:"Target store name (required)"`
		}) (string, error) {
			ss := memory.SessionStoresFromContext(ctx)
			if ss == nil {
				return "", fmt.Errorf("no memory stores mounted")
			}
			if req.Store == "" {
				return "", fmt.Errorf("store parameter is required")
			}
			st, found, writable := ss.FindStore(req.Store)
			if !found {
				return "", fmt.Errorf("store %q not found", req.Store)
			}
			if !writable {
				return "", fmt.Errorf("store %q is read-only in this session", req.Store)
			}
			if err := st.Delete(req.Filename); err != nil {
				return "", err
			}
			return fmt.Sprintf("Deleted %s from store %q.", req.Filename, req.Store), nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("memory tools: memory_delete: %w", err)
	}

	return []tool.BaseTool{listTool, readTool, searchTool, writeTool, editTool, deleteTool}, nil
}

func formatReadResult(doc memory.Document, storeName string) string {
	return fmt.Sprintf("store: %s\nfilename: %s\nsha256: %s\nversion: %d\n\n%s",
		storeName, doc.Filename, doc.SHA256, doc.Version, doc.Content)
}
