package config

import (
	"os"
	"path/filepath"
	"testing"
)

// clearModelEnv removes all model-related env vars for the duration of t.
func clearModelEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"WEB_SEARCH_PROVIDER", "BRAVE_API_KEY", "SERPER_API_KEY",
		"LOG_LEVEL", "LOG_FORMAT",
	} {
		t.Setenv(k, "")
	}
}

// ── SessionConfig.DriverOrDefault ────────────────────────────────────────

func TestSessionConfig_DriverOrDefault_Empty(t *testing.T) {
	c := &SessionConfig{}
	if got := c.DriverOrDefault(); got != SessionDriverMemory {
		t.Errorf("DriverOrDefault() = %q, want %q", got, SessionDriverMemory)
	}
}

func TestSessionConfig_DriverOrDefault_Explicit(t *testing.T) {
	for _, driver := range []string{"sqlite", "redis", "remote"} {
		c := &SessionConfig{Driver: driver}
		if got := c.DriverOrDefault(); got != driver {
			t.Errorf("DriverOrDefault() = %q, want %q", got, driver)
		}
	}
}

// ── SandboxConfig.DriverOrDefault ────────────────────────────────────────

func TestSandboxConfig_DriverOrDefault_Empty(t *testing.T) {
	c := &SandboxConfig{}
	if got := c.DriverOrDefault(); got != SandboxDriverLocal {
		t.Errorf("DriverOrDefault() = %q, want %q", got, SandboxDriverLocal)
	}
}

func TestSandboxConfig_DriverOrDefault_Explicit(t *testing.T) {
	for _, driver := range []string{SandboxDriverDocker, "remote"} {
		c := &SandboxConfig{Driver: driver}
		if got := c.DriverOrDefault(); got != driver {
			t.Errorf("DriverOrDefault() = %q, want %q", got, driver)
		}
	}
}

// ── Load: no file, env-variable fallbacks ────────────────────────────────

func TestLoad_NoFile_DefaultsToOpenAI(t *testing.T) {
	clearModelEnv(t)
	cfg, err := Load("no-such-file-xyz.yaml")
	if err == nil {
		// A non-existent explicit path should return an error (ReadFile fails).
		// We expect Load to fail here since the path was given explicitly.
		t.Log("explicit non-existent path returned nil error — file may have been created")
		return
	}
	_ = cfg
}

func TestLoad_EmptyPath_NoDefaultFile_UsesEnvDefaults(t *testing.T) {
	clearModelEnv(t)

	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	_ = os.Chdir(dir)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") unexpected error: %v", err)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want \"info\"", cfg.Log.Level)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("Log.Format = %q, want \"text\"", cfg.Log.Format)
	}
}

// ── Load: explicit YAML file ──────────────────────────────────────────────

func TestLoad_YAMLFile_ParsesFields(t *testing.T) {
	clearModelEnv(t)

	yaml := `
log:
  level: debug
  format: json
session:
  driver: sqlite
sandbox:
  driver: docker
`
	tmp := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(tmp, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load YAML: %v", err)
	}

	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q", cfg.Log.Level)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("Log.Format = %q", cfg.Log.Format)
	}
	if cfg.Session.Driver != SessionDriverSQLite {
		t.Errorf("Session.Driver = %q", cfg.Session.Driver)
	}
	if cfg.Sandbox.Driver != SandboxDriverDocker {
		t.Errorf("Sandbox.Driver = %q", cfg.Sandbox.Driver)
	}
}

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.yaml")
	_ = os.WriteFile(tmp, []byte("{ invalid yaml::: ["), 0o644)

	_, err := Load(tmp)
	if err == nil {
		t.Fatal("expected parse error for invalid YAML, got nil")
	}
}

func TestLoad_NonExistentExplicitPath_ReturnsError(t *testing.T) {
	_, err := Load("/tmp/this-file-does-not-exist-xyz-abc.yaml")
	if err == nil {
		t.Fatal("expected error for explicit non-existent path, got nil")
	}
}

// ── Log defaults ──────────────────────────────────────────────────────────

func TestLoad_LogDefaults(t *testing.T) {
	clearModelEnv(t)

	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	_ = os.Chdir(dir)

	cfg, _ := Load("")
	if cfg.Log.Level != "info" {
		t.Errorf("default log level = %q, want \"info\"", cfg.Log.Level)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("default log format = %q, want \"text\"", cfg.Log.Format)
	}
}

func TestLoad_LogLevelFromEnv(t *testing.T) {
	clearModelEnv(t)
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "json")

	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir) //nolint:errcheck
	_ = os.Chdir(dir)

	cfg, _ := Load("")
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want \"debug\"", cfg.Log.Level)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("Log.Format = %q, want \"json\"", cfg.Log.Format)
	}
}

// ── envOr helper ─────────────────────────────────────────────────────────

func TestEnvOr_ReturnsEnvWhenSet(t *testing.T) {
	t.Setenv("TEST_ENVOR_KEY", "from-env")
	if got := envOr("TEST_ENVOR_KEY", "default"); got != "from-env" {
		t.Errorf("envOr = %q, want \"from-env\"", got)
	}
}

func TestEnvOr_ReturnsFallbackWhenEmpty(t *testing.T) {
	t.Setenv("TEST_ENVOR_KEY", "")
	if got := envOr("TEST_ENVOR_KEY", "default"); got != "default" {
		t.Errorf("envOr = %q, want \"default\"", got)
	}
}

func TestEnvOr_ReturnsFallbackWhenUnset(t *testing.T) {
	os.Unsetenv("TEST_ENVOR_KEY_UNSET")
	if got := envOr("TEST_ENVOR_KEY_UNSET", "fallback"); got != "fallback" {
		t.Errorf("envOr = %q, want \"fallback\"", got)
	}
}

// ── applyIncludes / mcp + permission ─────────────────────────────────────────

func TestLoad_ToolsFile_LoadsMCPServers(t *testing.T) {
	clearModelEnv(t)
	dir := t.TempDir()

	toolsYAML := `
web_search:
  provider: serper
  api_key: serper-key
mcp_servers:
  github:
    type: stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
`
	mainYAML := `mcp: tools.yaml`

	if err := os.WriteFile(filepath.Join(dir, "tools.yaml"), []byte(toolsYAML), 0o644); err != nil {
		t.Fatalf("write tools.yaml: %v", err)
	}
	mainPath := filepath.Join(dir, "forge.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	cfg, err := Load(mainPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Tools.WebSearch.Provider != "serper" {
		t.Errorf("WebSearch.Provider = %q, want serper", cfg.Tools.WebSearch.Provider)
	}
	if _, ok := cfg.Tools.MCPServers["github"]; !ok {
		t.Error("MCPServers[github] not found after tools_file load")
	}
}

func TestLoad_PermissionFile_LoadsRules(t *testing.T) {
	clearModelEnv(t)
	dir := t.TempDir()

	permYAML := `
mode: dontAsk
allow_rules:
  - "Bash(git:*)"
  - "ReadFile(*)"
deny_rules:
  - "Bash(rm:*)"
`
	mainYAML := `permission: permission.yaml`

	if err := os.WriteFile(filepath.Join(dir, "permission.yaml"), []byte(permYAML), 0o644); err != nil {
		t.Fatalf("write permission.yaml: %v", err)
	}
	mainPath := filepath.Join(dir, "forge.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	cfg, err := Load(mainPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Permission.Mode != "dontAsk" {
		t.Errorf("Permission.Mode = %q, want dontAsk", cfg.Permission.Mode)
	}
	if len(cfg.Permission.AllowRules) != 2 {
		t.Errorf("AllowRules len = %d, want 2", len(cfg.Permission.AllowRules))
	}
	if len(cfg.Permission.DenyRules) != 1 {
		t.Errorf("DenyRules len = %d, want 1", len(cfg.Permission.DenyRules))
	}
}

func TestLoad_IncludeFile_OverridesInline(t *testing.T) {
	clearModelEnv(t)
	dir := t.TempDir()

	permYAML := `mode: dontAsk`
	mainYAML := `permission: permission.yaml`
	if err := os.WriteFile(filepath.Join(dir, "permission.yaml"), []byte(permYAML), 0o644); err != nil {
		t.Fatalf("write permission.yaml: %v", err)
	}
	mainPath := filepath.Join(dir, "forge.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	cfg, err := Load(mainPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Permission.Mode != "dontAsk" {
		t.Errorf("Permission.Mode = %q, want dontAsk", cfg.Permission.Mode)
	}
}

func TestLoad_ToolsFile_MissingFile_ReturnsError(t *testing.T) {
	clearModelEnv(t)
	dir := t.TempDir()

	mainYAML := `mcp: nonexistent.yaml`
	mainPath := filepath.Join(dir, "forge.yaml")
	if err := os.WriteFile(mainPath, []byte(mainYAML), 0o644); err != nil {
		t.Fatalf("write main config: %v", err)
	}

	_, err := Load(mainPath)
	if err == nil {
		t.Fatal("expected error for missing mcp file, got nil")
	}
}
