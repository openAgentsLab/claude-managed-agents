package memory

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"

	mem "forge/internal/memory"
)

// ── mock MemoryStore ──────────────────────────────────────────────────────────

type mockStore struct {
	name string
	desc string
	docs map[string]string
}

func newMock(name, desc string) *mockStore {
	return &mockStore{name: name, desc: desc, docs: make(map[string]string)}
}

func (m *mockStore) Name() string        { return m.name }
func (m *mockStore) Description() string { return m.desc }
func (m *mockStore) List() ([]mem.DocumentSummary, error) {
	out := make([]mem.DocumentSummary, 0, len(m.docs))
	for fn, c := range m.docs {
		exc := c
		if idx := strings.Index(c, "\n"); idx >= 0 {
			exc = c[:idx]
		}
		out = append(out, mem.DocumentSummary{Filename: fn, Excerpt: exc})
	}
	return out, nil
}
func (m *mockStore) Read(fn string) (mem.Document, bool, error) {
	c, ok := m.docs[fn]
	if !ok {
		return mem.Document{}, false, nil
	}
	return mem.Document{Filename: fn, Content: c}, true, nil
}
func (m *mockStore) Search(q string) ([]mem.SearchResult, error) {
	var results []mem.SearchResult
	for fn, c := range m.docs {
		if strings.Contains(c, q) {
			results = append(results, mem.SearchResult{StoreName: m.name, Filename: fn, Excerpt: c})
		}
	}
	return results, nil
}
func (m *mockStore) Write(fn, content, _ string) (string, error) {
	m.docs[fn] = content
	return "abc123", nil
}
func (m *mockStore) Edit(fn, old, newStr, _ string) (string, error) {
	c, ok := m.docs[fn]
	if !ok {
		return "", mem.ErrNotFound
	}
	m.docs[fn] = strings.ReplaceAll(c, old, newStr)
	return "def456", nil
}
func (m *mockStore) Delete(fn string) error {
	delete(m.docs, fn)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildCtx(user, project, tenant *mockStore, tenantWritable bool) context.Context {
	ss := mem.NewSession(user, project, tenant, tenantWritable)
	return mem.WithSessionStores(context.Background(), ss)
}

func findTool(t *testing.T, tools []tool.BaseTool, name string) tool.InvokableTool {
	t.Helper()
	for _, bt := range tools {
		info, err := bt.Info(context.Background())
		if err != nil || info == nil {
			continue
		}
		if info.Name == name {
			it, ok := bt.(tool.InvokableTool)
			if !ok {
				t.Fatalf("tool %q does not implement InvokableTool", name)
			}
			return it
		}
	}
	t.Fatalf("tool %q not found", name)
	return nil
}

func mustNewTools(t *testing.T) []tool.BaseTool {
	t.Helper()
	tools, err := NewTools()
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	return tools
}

func args(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ── NewTools ──────────────────────────────────────────────────────────────────

func TestNewTools_ReturnsSixTools(t *testing.T) {
	tools := mustNewTools(t)
	if len(tools) != 6 {
		t.Errorf("expected 6 tools, got %d", len(tools))
	}
}

func TestNewTools_ToolNames(t *testing.T) {
	tools := mustNewTools(t)
	names := map[string]bool{}
	for _, bt := range tools {
		info, err := bt.Info(context.Background())
		if err != nil || info == nil {
			t.Error("tool Info() returned nil or error")
			continue
		}
		names[info.Name] = true
	}
	expected := []string{"memory_list", "memory_read", "memory_search", "memory_write", "memory_edit", "memory_delete"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected tool %q", name)
		}
	}
}

// ── memory_list ───────────────────────────────────────────────────────────────

func TestMemoryList_NoStoresMounted(t *testing.T) {
	lt := findTool(t, mustNewTools(t), "memory_list")
	out, err := lt.InvokableRun(context.Background(), args(struct{}{}))
	if err != nil {
		t.Fatalf("memory_list: %v", err)
	}
	if !strings.Contains(out, "no memory stores") {
		t.Errorf("expected 'no memory stores' message; got %q", out)
	}
}

func TestMemoryList_EmptyStores(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	lt := findTool(t, mustNewTools(t), "memory_list")
	out, err := lt.InvokableRun(ctx, args(struct{}{}))
	if err != nil {
		t.Fatalf("memory_list: %v", err)
	}
	if !strings.Contains(out, "empty") {
		t.Errorf("expected 'empty' for stores with no docs; got %q", out)
	}
}

func TestMemoryList_ListsDocs(t *testing.T) {
	user := newMock("user", "user store")
	user.docs["notes.md"] = "my notes"
	ctx := buildCtx(user, newMock("project", ""), newMock("tenant", ""), false)
	lt := findTool(t, mustNewTools(t), "memory_list")
	out, err := lt.InvokableRun(ctx, args(struct{}{}))
	if err != nil {
		t.Fatalf("memory_list: %v", err)
	}
	if !strings.Contains(out, "notes.md") {
		t.Errorf("expected 'notes.md' in list output; got %q", out)
	}
}

// ── memory_read ───────────────────────────────────────────────────────────────

func TestMemoryRead_NoStoresMounted(t *testing.T) {
	rt := findTool(t, mustNewTools(t), "memory_read")
	_, err := rt.InvokableRun(context.Background(), args(map[string]string{"filename": "notes.md"}))
	if err == nil {
		t.Error("expected error when no stores mounted")
	}
}

func TestMemoryRead_DocFound(t *testing.T) {
	user := newMock("user", "")
	user.docs["notes.md"] = "hello world"
	ctx := buildCtx(user, newMock("project", ""), newMock("tenant", ""), false)
	rt := findTool(t, mustNewTools(t), "memory_read")
	out, err := rt.InvokableRun(ctx, args(map[string]string{"filename": "notes.md"}))
	if err != nil {
		t.Fatalf("memory_read: %v", err)
	}
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected doc content in output; got %q", out)
	}
}

func TestMemoryRead_DocNotFound(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	rt := findTool(t, mustNewTools(t), "memory_read")
	_, err := rt.InvokableRun(ctx, args(map[string]string{"filename": "missing.md"}))
	if err == nil {
		t.Error("expected error for missing document")
	}
	if !strings.Contains(err.Error(), "missing.md") {
		t.Errorf("error should mention filename; got %v", err)
	}
}

func TestMemoryRead_SpecificStore(t *testing.T) {
	user := newMock("user", "")
	user.docs["prefs.md"] = "user preferences"
	ctx := buildCtx(user, newMock("project", ""), newMock("tenant", ""), false)
	rt := findTool(t, mustNewTools(t), "memory_read")
	out, err := rt.InvokableRun(ctx, args(map[string]string{"filename": "prefs.md", "store": "user"}))
	if err != nil {
		t.Fatalf("memory_read with store: %v", err)
	}
	if !strings.Contains(out, "user preferences") {
		t.Errorf("expected content in output; got %q", out)
	}
}

func TestMemoryRead_StoreNotFound(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	rt := findTool(t, mustNewTools(t), "memory_read")
	_, err := rt.InvokableRun(ctx, args(map[string]string{"filename": "x.md", "store": "nonexistent"}))
	if err == nil {
		t.Error("expected error for unknown store")
	}
}

// ── memory_search ─────────────────────────────────────────────────────────────

func TestMemorySearch_NoStoresMounted(t *testing.T) {
	st := findTool(t, mustNewTools(t), "memory_search")
	out, err := st.InvokableRun(context.Background(), args(map[string]string{"query": "anything"}))
	if err != nil {
		t.Fatalf("memory_search: %v", err)
	}
	if !strings.Contains(out, "no memory stores") {
		t.Errorf("expected no-stores message; got %q", out)
	}
}

func TestMemorySearch_NoResults(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	st := findTool(t, mustNewTools(t), "memory_search")
	out, err := st.InvokableRun(ctx, args(map[string]string{"query": "xyz"}))
	if err != nil {
		t.Fatalf("memory_search: %v", err)
	}
	if !strings.Contains(out, "No results") {
		t.Errorf("expected 'No results'; got %q", out)
	}
}

func TestMemorySearch_MatchFound(t *testing.T) {
	user := newMock("user", "")
	user.docs["notes.md"] = "golang is great"
	ctx := buildCtx(user, newMock("project", ""), newMock("tenant", ""), false)
	st := findTool(t, mustNewTools(t), "memory_search")
	out, err := st.InvokableRun(ctx, args(map[string]string{"query": "golang"}))
	if err != nil {
		t.Fatalf("memory_search: %v", err)
	}
	if !strings.Contains(out, "notes.md") {
		t.Errorf("expected 'notes.md' in search results; got %q", out)
	}
}

func TestMemorySearch_StoreNotFound(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	st := findTool(t, mustNewTools(t), "memory_search")
	_, err := st.InvokableRun(ctx, args(map[string]string{"query": "x", "store": "ghost"}))
	if err == nil {
		t.Error("expected error for unknown store")
	}
}

// ── memory_write ──────────────────────────────────────────────────────────────

func TestMemoryWrite_NoStoresMounted(t *testing.T) {
	wt := findTool(t, mustNewTools(t), "memory_write")
	_, err := wt.InvokableRun(context.Background(), args(map[string]string{
		"filename": "notes.md", "content": "hello", "store": "user",
	}))
	if err == nil {
		t.Error("expected error when no stores mounted")
	}
}

func TestMemoryWrite_Success(t *testing.T) {
	user := newMock("user", "")
	ctx := buildCtx(user, newMock("project", ""), newMock("tenant", ""), false)
	wt := findTool(t, mustNewTools(t), "memory_write")
	out, err := wt.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "content": "hello world", "store": "user",
	}))
	if err != nil {
		t.Fatalf("memory_write: %v", err)
	}
	if !strings.Contains(out, "notes.md") {
		t.Errorf("expected filename in output; got %q", out)
	}
	if user.docs["notes.md"] != "hello world" {
		t.Error("content not persisted to mock store")
	}
}

func TestMemoryWrite_EmptyContent(t *testing.T) {
	user := newMock("user", "")
	ctx := buildCtx(user, newMock("project", ""), newMock("tenant", ""), false)
	wt := findTool(t, mustNewTools(t), "memory_write")
	_, err := wt.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "content": "   ", "store": "user",
	}))
	if err == nil {
		t.Error("expected error for empty content")
	}
}

func TestMemoryWrite_MissingStore(t *testing.T) {
	user := newMock("user", "")
	ctx := buildCtx(user, newMock("project", ""), newMock("tenant", ""), false)
	wt := findTool(t, mustNewTools(t), "memory_write")
	_, err := wt.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "content": "hello", "store": "",
	}))
	if err == nil {
		t.Error("expected error for missing store parameter")
	}
}

func TestMemoryWrite_StoreNotFound(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	wt := findTool(t, mustNewTools(t), "memory_write")
	_, err := wt.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "content": "hello", "store": "ghost",
	}))
	if err == nil {
		t.Error("expected error for nonexistent store")
	}
}

func TestMemoryWrite_ReadOnlyStore(t *testing.T) {
	// tenant is mounted read-only (tenantWritable=false)
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	wt := findTool(t, mustNewTools(t), "memory_write")
	_, err := wt.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "content": "hello", "store": "tenant",
	}))
	if err == nil {
		t.Error("expected error for read-only store")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("error should mention read-only; got %v", err)
	}
}

// ── memory_edit ───────────────────────────────────────────────────────────────

func TestMemoryEdit_NoStoresMounted(t *testing.T) {
	et := findTool(t, mustNewTools(t), "memory_edit")
	_, err := et.InvokableRun(context.Background(), args(map[string]string{
		"filename": "notes.md", "old_str": "old", "new_str": "new", "store": "user",
	}))
	if err == nil {
		t.Error("expected error when no stores mounted")
	}
}

func TestMemoryEdit_Success(t *testing.T) {
	user := newMock("user", "")
	user.docs["notes.md"] = "hello world"
	ctx := buildCtx(user, newMock("project", ""), newMock("tenant", ""), false)
	et := findTool(t, mustNewTools(t), "memory_edit")
	out, err := et.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "old_str": "world", "new_str": "there", "store": "user",
	}))
	if err != nil {
		t.Fatalf("memory_edit: %v", err)
	}
	if !strings.Contains(out, "notes.md") {
		t.Errorf("expected filename in output; got %q", out)
	}
	if !strings.Contains(user.docs["notes.md"], "there") {
		t.Errorf("edit not applied; content: %q", user.docs["notes.md"])
	}
}

func TestMemoryEdit_MissingStore(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	et := findTool(t, mustNewTools(t), "memory_edit")
	_, err := et.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "old_str": "x", "new_str": "y", "store": "",
	}))
	if err == nil {
		t.Error("expected error for missing store parameter")
	}
}

func TestMemoryEdit_StoreNotFound(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	et := findTool(t, mustNewTools(t), "memory_edit")
	_, err := et.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "old_str": "x", "new_str": "y", "store": "ghost",
	}))
	if err == nil {
		t.Error("expected error for nonexistent store")
	}
}

func TestMemoryEdit_ReadOnlyStore(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	et := findTool(t, mustNewTools(t), "memory_edit")
	_, err := et.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "old_str": "x", "new_str": "y", "store": "tenant",
	}))
	if err == nil {
		t.Error("expected error for read-only store")
	}
}

// ── memory_delete ─────────────────────────────────────────────────────────────

func TestMemoryDelete_NoStoresMounted(t *testing.T) {
	dt := findTool(t, mustNewTools(t), "memory_delete")
	_, err := dt.InvokableRun(context.Background(), args(map[string]string{
		"filename": "notes.md", "store": "user",
	}))
	if err == nil {
		t.Error("expected error when no stores mounted")
	}
}

func TestMemoryDelete_Success(t *testing.T) {
	user := newMock("user", "")
	user.docs["notes.md"] = "will be deleted"
	ctx := buildCtx(user, newMock("project", ""), newMock("tenant", ""), false)
	dt := findTool(t, mustNewTools(t), "memory_delete")
	out, err := dt.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "store": "user",
	}))
	if err != nil {
		t.Fatalf("memory_delete: %v", err)
	}
	if !strings.Contains(out, "notes.md") {
		t.Errorf("expected filename in output; got %q", out)
	}
	if _, exists := user.docs["notes.md"]; exists {
		t.Error("document should have been deleted from mock store")
	}
}

func TestMemoryDelete_MissingStore(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	dt := findTool(t, mustNewTools(t), "memory_delete")
	_, err := dt.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "store": "",
	}))
	if err == nil {
		t.Error("expected error for missing store parameter")
	}
}

func TestMemoryDelete_StoreNotFound(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	dt := findTool(t, mustNewTools(t), "memory_delete")
	_, err := dt.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "store": "ghost",
	}))
	if err == nil {
		t.Error("expected error for nonexistent store")
	}
}

func TestMemoryDelete_ReadOnlyStore(t *testing.T) {
	ctx := buildCtx(newMock("user", ""), newMock("project", ""), newMock("tenant", ""), false)
	dt := findTool(t, mustNewTools(t), "memory_delete")
	_, err := dt.InvokableRun(ctx, args(map[string]string{
		"filename": "notes.md", "store": "tenant",
	}))
	if err == nil {
		t.Error("expected error for read-only store")
	}
}
