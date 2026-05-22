package memory

import (
	"context"
	"strings"
	"testing"
)

// ── mock MemoryStore ──────────────────────────────────────────────────────────

type mockStore struct {
	name string
	desc string
	docs map[string]string // filename → content
}

func newMock(name, desc string) *mockStore {
	return &mockStore{name: name, desc: desc, docs: make(map[string]string)}
}

func (m *mockStore) Name() string        { return m.name }
func (m *mockStore) Description() string { return m.desc }
func (m *mockStore) List() ([]DocumentSummary, error) {
	out := make([]DocumentSummary, 0, len(m.docs))
	for fn, c := range m.docs {
		exc := c
		if idx := strings.Index(c, "\n"); idx >= 0 {
			exc = c[:idx]
		}
		out = append(out, DocumentSummary{Filename: fn, Excerpt: exc})
	}
	return out, nil
}
func (m *mockStore) Read(fn string) (Document, bool, error) {
	c, ok := m.docs[fn]
	if !ok {
		return Document{}, false, nil
	}
	return Document{Filename: fn, Content: c}, true, nil
}
func (m *mockStore) Search(q string) ([]SearchResult, error) { return nil, nil }
func (m *mockStore) Write(fn, content, _ string) (string, error) {
	m.docs[fn] = content
	return "sha", nil
}
func (m *mockStore) Edit(fn, old, new, _ string) (string, error) {
	c, ok := m.docs[fn]
	if !ok {
		return "", ErrNotFound
	}
	m.docs[fn] = strings.ReplaceAll(c, old, new)
	return "sha", nil
}
func (m *mockStore) Delete(fn string) error {
	delete(m.docs, fn)
	return nil
}

// ── ValidateFilename ──────────────────────────────────────────────────────────

func TestValidateFilename_Valid(t *testing.T) {
	valid := []string{"notes.md", "my-notes.md", "project_123.md", "a.md"}
	for _, fn := range valid {
		if err := ValidateFilename(fn); err != nil {
			t.Errorf("ValidateFilename(%q): unexpected error: %v", fn, err)
		}
	}
}

func TestValidateFilename_Invalid(t *testing.T) {
	invalid := []string{"Notes.md", "my notes.md", "file.txt", "sub/dir.md", ".md", ""}
	for _, fn := range invalid {
		if err := ValidateFilename(fn); err == nil {
			t.Errorf("ValidateFilename(%q): expected error, got nil", fn)
		}
	}
}

// ── ValidateContent ───────────────────────────────────────────────────────────

func TestValidateContent_WithinLimit(t *testing.T) {
	if err := ValidateContent("hello world"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateContent_ExceedsLimit(t *testing.T) {
	big := strings.Repeat("x", MaxDocumentBytes+1)
	if err := ValidateContent(big); err == nil {
		t.Error("expected error for content exceeding MaxDocumentBytes, got nil")
	}
}

func TestValidateContent_ExactLimit(t *testing.T) {
	exact := strings.Repeat("x", MaxDocumentBytes)
	if err := ValidateContent(exact); err != nil {
		t.Errorf("exact-limit content should be valid: %v", err)
	}
}

// ── validateVisibility ────────────────────────────────────────────────────────

func TestValidateVisibility_Valid(t *testing.T) {
	for _, v := range []string{"private", "shared_tenant"} {
		if err := validateVisibility(v); err != nil {
			t.Errorf("validateVisibility(%q): unexpected error: %v", v, err)
		}
	}
}

func TestValidateVisibility_Invalid(t *testing.T) {
	for _, v := range []string{"public", "global", "", "shared_project"} {
		if err := validateVisibility(v); err == nil {
			t.Errorf("validateVisibility(%q): expected error, got nil", v)
		}
	}
}

// ── validateWritePolicy ───────────────────────────────────────────────────────

func TestValidateWritePolicy_Valid(t *testing.T) {
	for _, wp := range []string{"owner_only", "members"} {
		if err := validateWritePolicy(wp); err != nil {
			t.Errorf("validateWritePolicy(%q): unexpected error: %v", wp, err)
		}
	}
}

func TestValidateWritePolicy_Invalid(t *testing.T) {
	for _, wp := range []string{"everyone", "admin_only", ""} {
		if err := validateWritePolicy(wp); err == nil {
			t.Errorf("validateWritePolicy(%q): expected error, got nil", wp)
		}
	}
}

// ── context ───────────────────────────────────────────────────────────────────

func TestWithSessionStores_RoundTrip(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, false)

	ctx := WithSessionStores(context.Background(), ss)
	got := SessionStoresFromContext(ctx)
	if got != ss {
		t.Error("SessionStoresFromContext should return the same *SessionStores")
	}
}

func TestSessionStoresFromContext_NilWhenAbsent(t *testing.T) {
	got := SessionStoresFromContext(context.Background())
	if got != nil {
		t.Error("expected nil when no SessionStores in context")
	}
}

func TestWithSystemContext_RoundTrip(t *testing.T) {
	ctx := WithSystemContext(context.Background(), "# Memory\n\nsome text")
	got := SystemContextFromContext(ctx)
	if got != "# Memory\n\nsome text" {
		t.Errorf("got %q", got)
	}
}

func TestSystemContextFromContext_EmptyWhenAbsent(t *testing.T) {
	got := SystemContextFromContext(context.Background())
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// ── SessionStores ─────────────────────────────────────────────────────────────

func TestNewSession_ThreeMounts(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, false)
	if len(ss.MountedStores()) != 3 {
		t.Errorf("expected 3 mounts, got %d", len(ss.MountedStores()))
	}
}

func TestNewSession_UserAndProjectWritable(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, false)
	mounted := ss.MountedStores()
	if !mounted[0].Writable {
		t.Error("user store should be writable")
	}
	if !mounted[1].Writable {
		t.Error("project store should be writable")
	}
	if mounted[2].Writable {
		t.Error("tenant store should be read-only when tenantWritable=false")
	}
}

func TestNewSession_TenantWritable(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, true)
	mounted := ss.MountedStores()
	if !mounted[2].Writable {
		t.Error("tenant store should be writable when tenantWritable=true")
	}
}

func TestMount_AddsStore(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, false)

	custom := newMock("custom-store", "my custom store")
	if err := ss.Mount(custom, true); err != nil {
		t.Fatalf("Mount: unexpected error: %v", err)
	}
	if len(ss.MountedStores()) != 4 {
		t.Errorf("expected 4 mounts after Mount, got %d", len(ss.MountedStores()))
	}
}

func TestMount_ExceedsMaxReturnsError(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, false)

	for i := 0; i < MaxMountedStores-3; i++ {
		extra := newMock("extra-"+string(rune('a'+i)), "desc")
		if err := ss.Mount(extra, false); err != nil {
			t.Fatalf("Mount %d: unexpected error: %v", i, err)
		}
	}
	overflow := newMock("overflow", "desc")
	if err := ss.Mount(overflow, false); err == nil {
		t.Error("expected error when exceeding MaxMountedStores, got nil")
	}
}

func TestFindStore_ByName(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, false)

	s, found, writable := ss.FindStore(UserStoreName)
	if !found {
		t.Fatal("expected to find user store")
	}
	if s.Name() != UserStoreName {
		t.Errorf("store name: got %q", s.Name())
	}
	if !writable {
		t.Error("user store should be writable")
	}
}

func TestFindStore_NotFound(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, false)

	_, found, _ := ss.FindStore("nonexistent")
	if found {
		t.Error("expected not found for nonexistent store name")
	}
}

// ── readOnlyStore ─────────────────────────────────────────────────────────────

func TestReadOnlyStore_WriteReturnsErrForbidden(t *testing.T) {
	inner := newMock("tenant", "tenant store")
	ros := &readOnlyStore{inner}
	_, err := ros.Write("notes.md", "content", "")
	if err != ErrForbidden {
		t.Errorf("Write: expected ErrForbidden, got %v", err)
	}
}

func TestReadOnlyStore_EditReturnsErrForbidden(t *testing.T) {
	inner := newMock("tenant", "tenant store")
	ros := &readOnlyStore{inner}
	_, err := ros.Edit("notes.md", "old", "new", "")
	if err != ErrForbidden {
		t.Errorf("Edit: expected ErrForbidden, got %v", err)
	}
}

func TestReadOnlyStore_DeleteReturnsErrForbidden(t *testing.T) {
	inner := newMock("tenant", "tenant store")
	ros := &readOnlyStore{inner}
	if err := ros.Delete("notes.md"); err != ErrForbidden {
		t.Errorf("Delete: expected ErrForbidden, got %v", err)
	}
}

func TestReadOnlyStore_ReadStillWorks(t *testing.T) {
	inner := newMock("tenant", "tenant store")
	inner.docs["notes.md"] = "hello"
	ros := &readOnlyStore{inner}
	doc, found, err := ros.Read("notes.md")
	if err != nil || !found || doc.Content != "hello" {
		t.Errorf("Read on readOnlyStore: err=%v found=%v content=%q", err, found, doc.Content)
	}
}

// ── MergeList ─────────────────────────────────────────────────────────────────

func TestMergeList_ContainsStoreNames(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	user.docs["notes.md"] = "first line\nsecond line"
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, false)

	list := ss.MergeList()
	if !strings.Contains(list, UserStoreName) {
		t.Errorf("MergeList should contain user store name; got:\n%s", list)
	}
	if !strings.Contains(list, "notes.md") {
		t.Errorf("MergeList should list document filenames; got:\n%s", list)
	}
	if !strings.Contains(list, "(empty)") {
		t.Errorf("MergeList should mark empty stores; got:\n%s", list)
	}
}

// ── BuildSystemContext ────────────────────────────────────────────────────────

func TestBuildSystemContext_ContainsStoreNames(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, false)

	ctx := ss.BuildSystemContext()
	if !strings.Contains(ctx, "# Memory") {
		t.Error("BuildSystemContext should contain '# Memory' header")
	}
	for _, name := range []string{UserStoreName, ProjectStoreName, TenantStoreName} {
		if !strings.Contains(ctx, name) {
			t.Errorf("BuildSystemContext should contain store name %q", name)
		}
	}
}

func TestBuildSystemContext_ReadOnlyAnnotation(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, false)

	ctx := ss.BuildSystemContext()
	if !strings.Contains(ctx, "(read-only)") {
		t.Error("BuildSystemContext should annotate tenant store as (read-only) when tenantWritable=false")
	}
}

func TestBuildSystemContext_WritableTenantNoAnnotation(t *testing.T) {
	user := newMock(UserStoreName, UserStoreDesc)
	proj := newMock(ProjectStoreName, ProjectStoreDesc)
	tenant := newMock(TenantStoreName, TenantStoreDesc)
	ss := NewSession(user, proj, tenant, true)

	ctx := ss.BuildSystemContext()
	if strings.Contains(ctx, "(read-only)") {
		t.Error("BuildSystemContext should not annotate (read-only) when tenantWritable=true")
	}
}

// ── builtin constants ─────────────────────────────────────────────────────────

func TestBuiltinStoreNames_Distinct(t *testing.T) {
	names := []string{UserStoreName, ProjectStoreName, TenantStoreName}
	seen := make(map[string]bool)
	for _, n := range names {
		if n == "" {
			t.Error("store name constant must not be empty")
		}
		if seen[n] {
			t.Errorf("duplicate store name constant: %q", n)
		}
		seen[n] = true
	}
}
