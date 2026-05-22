package resources

import (
	"testing"
)

// ── SafeJoin ──────────────────────────────────────────────────────────────────

func TestSafeJoin_Normal(t *testing.T) {
	root := "/workspace"
	got, err := SafeJoin(root, "subdir/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/workspace/subdir/file.txt" {
		t.Errorf("got %q, want %q", got, "/workspace/subdir/file.txt")
	}
}

func TestSafeJoin_EmptyRel(t *testing.T) {
	_, err := SafeJoin("/workspace", "")
	if err == nil {
		t.Fatal("expected error for empty rel, got nil")
	}
}

func TestSafeJoin_AbsoluteRel(t *testing.T) {
	_, err := SafeJoin("/workspace", "/etc/passwd")
	if err == nil {
		t.Fatal("expected error for absolute rel path, got nil")
	}
}

func TestSafeJoin_PathTraversal(t *testing.T) {
	_, err := SafeJoin("/workspace", "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func TestSafeJoin_NestedRelative(t *testing.T) {
	root := "/workspace"
	got, err := SafeJoin(root, "a/b/c.py")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/workspace/a/b/c.py" {
		t.Errorf("got %q, want %q", got, "/workspace/a/b/c.py")
	}
}

func TestSafeJoin_DotSlash(t *testing.T) {
	// ./file.txt is valid — stays within root
	root := "/workspace"
	got, err := SafeJoin(root, "file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/workspace/file.txt" {
		t.Errorf("got %q", got)
	}
}

// ── EmbedToken ────────────────────────────────────────────────────────────────

func TestEmbedToken_HTTPS(t *testing.T) {
	url := "https://github.com/org/repo"
	got := EmbedToken(url, "mytoken")
	want := "https://x-token:mytoken@github.com/org/repo"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEmbedToken_NonHTTPS(t *testing.T) {
	url := "git@github.com:org/repo.git"
	got := EmbedToken(url, "mytoken")
	if got != url {
		t.Errorf("non-HTTPS URL should be unchanged; got %q", got)
	}
}

func TestEmbedToken_HTTP(t *testing.T) {
	url := "http://example.com/repo"
	got := EmbedToken(url, "tok")
	// http:// does not match the "https://" prefix, so unchanged
	if got != url {
		t.Errorf("http URL should be unchanged; got %q", got)
	}
}

func TestEmbedToken_EmptyToken(t *testing.T) {
	url := "https://github.com/org/repo"
	got := EmbedToken(url, "")
	want := "https://x-token:@github.com/org/repo"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ── PackageSpec.IsEmpty ───────────────────────────────────────────────────────

func TestPackageSpec_IsEmpty_True(t *testing.T) {
	p := PackageSpec{}
	if !p.IsEmpty() {
		t.Error("zero-value PackageSpec should be empty")
	}
}

func TestPackageSpec_IsEmpty_Pip(t *testing.T) {
	p := PackageSpec{Pip: []string{"requests"}}
	if p.IsEmpty() {
		t.Error("PackageSpec with pip packages should not be empty")
	}
}

func TestPackageSpec_IsEmpty_Npm(t *testing.T) {
	p := PackageSpec{Npm: []string{"lodash"}}
	if p.IsEmpty() {
		t.Error("PackageSpec with npm packages should not be empty")
	}
}

func TestPackageSpec_IsEmpty_Apt(t *testing.T) {
	p := PackageSpec{Apt: []string{"curl"}}
	if p.IsEmpty() {
		t.Error("PackageSpec with apt packages should not be empty")
	}
}

func TestPackageSpec_IsEmpty_Cargo(t *testing.T) {
	p := PackageSpec{Cargo: []string{"serde"}}
	if p.IsEmpty() {
		t.Error("PackageSpec with cargo packages should not be empty")
	}
}

// ── NetworkConfig ─────────────────────────────────────────────────────────────

func TestNetworkingConstants(t *testing.T) {
	if NetworkingUnrestricted == NetworkingLimited {
		t.Error("networking constants must be distinct")
	}
	if NetworkingUnrestricted == "" || NetworkingLimited == "" {
		t.Error("networking constants must not be empty")
	}
}
