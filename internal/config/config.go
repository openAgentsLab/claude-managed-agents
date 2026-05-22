package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Session driver names.
const (
	SessionDriverMemory   = "memory"
	SessionDriverSQLite   = "sqlite"
	SessionDriverPostgres = "postgres"
)

// Sandbox driver names.
const (
	SandboxDriverLocal  = "local"
	SandboxDriverDocker = "docker"
	SandboxDriverK8s    = "k8s"
)

// ContainerWorkspaceRoot is the fixed path inside every Docker/K8s sandbox
// container where the per-user workspace directory is bind-mounted.
const ContainerWorkspaceRoot = "/workspace"

// StorageConfig defines named storage instances.
// Subsystems reference an instance with "type.name" driver syntax, e.g.
// session.driver: "postgres.default" or session.driver: "redis.cache".
type StorageConfig struct {
	Postgres map[string]PostgresInstance `yaml:"postgres,omitempty"`
	Redis    map[string]RedisInstance    `yaml:"redis,omitempty"`
}

// PostgresInstance holds connection parameters for one PostgreSQL instance.
type PostgresInstance struct {
	// DSN is the connection string. Env fallback for "default": FORGE_STORAGE_DSN.
	DSN string `yaml:"dsn"`
}

// RedisInstance holds connection parameters for one Redis instance.
type RedisInstance struct {
	// Addr is the Redis host:port. Env fallback for "default": FORGE_REDIS_ADDR.
	Addr string `yaml:"addr"`
	// Password is optional. Env fallback for "default": FORGE_REDIS_PASSWORD.
	Password string `yaml:"password"`
	// DB is the Redis database number (default 0).
	DB int `yaml:"db"`
}

const defaultEmbeddedWorkerConcurrency = 2

// WorkerConfig controls the embedded and standalone worker pools.
type WorkerConfig struct {
	// Concurrency is the number of worker goroutines. Default: 2.
	Concurrency int `yaml:"concurrency"`
	// HealthAddr is the listen address for the standalone worker's HTTP health
	// probe server (e.g. ":8082"). Empty disables the server.
	HealthAddr string `yaml:"health_addr"`
}

func (c *WorkerConfig) ConcurrencyOrDefault() int {
	if c.Concurrency > 0 {
		return c.Concurrency
	}
	return defaultEmbeddedWorkerConcurrency
}

// EventBusConfig selects the pub/sub backend for SSE event distribution and
// cross-node interrupt signals.
type EventBusConfig struct {
	// Driver is "memory" (default, single-node) or "redis" (multi-node).
	Driver string `yaml:"driver"`
	// Redis names the storage.redis instance to use when driver is "redis".
	// Defaults to "default" when unset.
	Redis string `yaml:"redis"`
}

func (c EventBusConfig) DriverOrDefault() string {
	if c.Driver == "" {
		return "memory"
	}
	return c.Driver
}

func (c EventBusConfig) RedisNameOrDefault() string {
	if c.Redis == "" {
		return "default"
	}
	return c.Redis
}

// Config is the top-level service configuration.
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Log        LogConfig        `yaml:"log"`
	Tools      ToolsConfig      `yaml:"tools"`
	Session    SessionConfig    `yaml:"session"`
	Sandbox    SandboxConfig    `yaml:"sandbox"`
	Memory     MemoryConfig     `yaml:"memory"`
	Auth       AuthConfig       `yaml:"auth"`
	Storage    StorageConfig    `yaml:"storage"`
	Tasks      TasksConfig      `yaml:"tasks"`
	Store      StoreConfig      `yaml:"store"`
	Worker     WorkerConfig     `yaml:"worker"`
	EventBus   EventBusConfig   `yaml:"event_bus"`

	// MCP points to a separate YAML file whose top-level fields map to
	// ToolsConfig (web_search, mcp_servers).  Relative paths are resolved
	// against the directory of the main config file.  Values in the included
	// file take precedence over any inline tools: block in this file.
	MCP string `yaml:"mcp,omitempty"`
	// PermissionFile points to a separate YAML file whose top-level fields map
	// to PermissionConfig (mode, allow_rules, deny_rules).  Inline permission:
	// blocks in the main config file are not supported; always use this field.
	PermissionFile string `yaml:"permission,omitempty"`

	// Permission holds the loaded permission config (populated from PermissionFile).
	// yaml:"-" prevents the main config file from overriding it directly.
	Permission PermissionConfig `yaml:"-"`
}

// ResourceQuota defines per-tenant Docker container resource limits.
// Zero values mean unlimited (Docker default).
type ResourceQuota struct {
	// MemoryBytes is the memory limit in bytes. 0 = unlimited.
	MemoryBytes int64 `yaml:"memory_bytes" json:"memory_bytes"`
	// NanoCPUs is the CPU limit in nano CPUs. 1 CPU = 1_000_000_000. 0 = unlimited.
	NanoCPUs int64 `yaml:"nano_cpus" json:"nano_cpus"`
}

// ModelOverride lets a tenant configure a different LLM endpoint or model.
// Non-empty fields replace the corresponding global ModelConfig value; empty
// fields fall back to the global value.
type ModelOverride struct {
	Provider   string `yaml:"provider,omitempty" json:"provider,omitempty"`
	APIKey     string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	BaseURL    string `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	Model      string `yaml:"model,omitempty" json:"model,omitempty"`
	ByAzure    bool   `yaml:"by_azure,omitempty" json:"by_azure,omitempty"`
	APIVersion string `yaml:"api_version,omitempty" json:"api_version,omitempty"`
}

// BrainOverride lets a tenant tune inference behaviour.
// Non-empty / non-zero fields replace the corresponding DefaultBrainConfig value.
// Model selection belongs in ModelOverride, not here.
type BrainOverride struct {
	// Effort overrides the token-budget effort level: max | high | medium | low.
	Effort string `yaml:"effort,omitempty" json:"effort,omitempty"`
	// Thinking overrides adaptive thinking: "adaptive" | "disabled".
	Thinking string `yaml:"thinking,omitempty" json:"thinking,omitempty"`
	// MaxRetries overrides the 429/529 retry limit.
	MaxRetries int `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
}

// TenantSettings holds the permission policy and resource limits for a tenant.
// These act as an upper bound for all users within the tenant.
type TenantSettings struct {
	// AllowRules are tenant-level allow rules applied on top of global rules.
	AllowRules []string `yaml:"allow_rules,omitempty" json:"allow_rules,omitempty"`
	// DenyRules are tenant-level deny rules applied on top of global rules.
	DenyRules []string `yaml:"deny_rules,omitempty" json:"deny_rules,omitempty"`
	// ResourceQuota sets Docker container CPU/memory limits for this tenant.
	ResourceQuota ResourceQuota `yaml:"resource_quota" json:"resource_quota"`
	// ModelOverride optionally replaces the global model config for this tenant.
	ModelOverride *ModelOverride `yaml:"model,omitempty" json:"model,omitempty"`
	// BrainOverride optionally tunes inference parameters for this tenant.
	BrainOverride *BrainOverride `yaml:"brain,omitempty" json:"brain,omitempty"`
}

// AuthConfig controls authentication for the HTTP serve mode.
type AuthConfig struct {
	// JWTSecret is the HS256 signing secret. Must be set for serve mode.
	// Env fallback: FORGE_JWT_SECRET.
	JWTSecret string `yaml:"jwt_secret"`
	// TokenTTLHours sets how long a login-issued JWT is valid. Default: 24.
	TokenTTLHours int `yaml:"token_ttl_hours"`
}


// TasksConfig controls the task store driver.
type TasksConfig struct {
	// Driver selects the task store backend: "sqlite" (default) or "postgres".
	Driver  string            `yaml:"driver"`
	Options map[string]string `yaml:"options,omitempty"`
}

// DriverOrDefault returns the configured driver, defaulting to "sqlite".
func (c *TasksConfig) DriverOrDefault() string {
	if c.Driver == "" {
		return "sqlite"
	}
	return c.Driver
}

// StoreConfig controls the app store driver (tenants, users, and future config entities).
type StoreConfig struct {
	// Driver selects the app store backend: "sqlite" (default) or "postgres".
	Driver  string            `yaml:"driver"`
	Options map[string]string `yaml:"options,omitempty"`
}

// DriverOrDefault returns the configured driver, defaulting to "sqlite".
func (c *StoreConfig) DriverOrDefault() string {
	if c.Driver == "" {
		return "sqlite"
	}
	return c.Driver
}

// applyEnv fills empty AuthConfig fields from environment variables.
func (a *AuthConfig) applyEnv() {
	if a.JWTSecret == "" {
		a.JWTSecret = os.Getenv("FORGE_JWT_SECRET")
	}
}

// MemoryConfig controls the memory store system.
type MemoryConfig struct {
	// Disabled turns off the memory system entirely. Default false (enabled).
	Disabled bool              `yaml:"disabled"`
	// Driver selects the backend: "sqlite" (default) or "postgres".
	Driver   string            `yaml:"driver"`
	Options  map[string]string `yaml:"options,omitempty"`
}

// PermissionConfig controls the permission system behaviour.
type PermissionConfig struct {
	// Mode sets the global permission mode (e.g. "dontAsk").
	Mode string `yaml:"mode,omitempty"`
	// AllowRules is a list of allow rules applied globally.
	// Format: "ToolName(pattern)" e.g. "Bash(git:*)".
	AllowRules []string `yaml:"allow_rules,omitempty"`
	// DenyRules is a list of deny rules applied globally.
	// Format: "ToolName(pattern)" e.g. "Bash(rm:*)".
	DenyRules []string `yaml:"deny_rules,omitempty"`
}

// LogConfig controls the application logger.
type LogConfig struct {
	// Level sets the minimum log level: debug, info, warn, error (default: info).
	Level string `yaml:"level"`
	// Format selects the output format: text (default) or json.
	Format string `yaml:"format"`
	// File is the path to write logs to. If empty, logs are written to stderr.
	File string `yaml:"file"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	HTTPAddr string `yaml:"http_addr"`
}

// ModelConfig selects the LLM provider and credentials.
// Values are loaded from the config file first; any empty field
// then falls back to the corresponding environment variable.
//
// Supported providers:
//
//	openai     – OpenAI or any OpenAI-compatible endpoint (default)
//	anthropic  – Anthropic via the Messages API
type ModelConfig struct {
	// Provider selects the LLM backend: "openai" (default) or "anthropic".
	Provider string `yaml:"provider"`
	// APIKey is the API key for the selected provider.
	APIKey string `yaml:"api_key"`
	// BaseURL overrides the default API endpoint (OpenAI / compatible only).
	BaseURL string `yaml:"base_url"`
	// Model is the model name to use.
	Model string `yaml:"model"`
	// ByAzure enables Azure OpenAI mode (openai only).
	ByAzure bool `yaml:"by_azure"`
	// APIVersion is the Azure API version (openai/Azure only).
	APIVersion string `yaml:"api_version"`
	// MaxTokens overrides the default output token cap (0 = use provider default).
	// Lower values reduce latency for short-output use cases.
	MaxTokens int `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
}

// MCPServerConfig describes a single MCP server to connect at agent startup.
// Values are loaded from the config file and are not per-user (for per-user
// MCP servers, use the /v1/mcp/servers API in serve mode).
type MCPServerConfig struct {
	Type     string            `yaml:"type"`               // stdio | sse | http | ws; default "stdio"
	Command  string            `yaml:"command,omitempty"`
	Args     []string          `yaml:"args,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	URL      string            `yaml:"url,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty"`
	Disabled bool              `yaml:"disabled,omitempty"`
}

// ToolsConfig holds optional tool-specific configuration.
type ToolsConfig struct {
	WebSearch  WebSearchConfig             `yaml:"web_search"`
	MCPServers map[string]MCPServerConfig  `yaml:"mcp_servers,omitempty"`
}

// WebSearchConfig configures the web_search tool backend.
type WebSearchConfig struct {
	// Provider selects the search backend: "brave" (default) or "serper".
	Provider string `yaml:"provider"`
	// APIKey for the selected provider.
	// Env fallback: BRAVE_API_KEY (brave) or SERPER_API_KEY (serper).
	APIKey string `yaml:"api_key"`
}

// SessionConfig controls Session storage driver selection.
// driver corresponds to the name registered via session.Register();
// options are driver-private key-value config, passed through to Factory.
type SessionConfig struct {
	Driver  string            `yaml:"driver"`            // memory (default) | sqlite | redis | remote | ...
	Options map[string]string `yaml:"options,omitempty"` // e.g. addr: "http://session-svc:8080"
}

// DriverOrDefault returns the configured driver name, defaulting to SessionDriverMemory.
func (c *SessionConfig) DriverOrDefault() string {
	if c.Driver == "" {
		return SessionDriverMemory
	}
	return c.Driver
}

// SandboxConfig controls Sandbox execution environment driver selection.
type SandboxConfig struct {
	Driver  string            `yaml:"driver"`            // local (default) | docker | k8s
	Options map[string]string `yaml:"options,omitempty"` // driver-specific config

	// VolumesRoot is the host-side root directory shared between the
	// orchestration service and sandbox containers (NFS mount or shared Docker
	// volume). Required for Docker; enables dynamic resource mounting and output
	// collection. Leave empty only for the local driver.
	VolumesRoot string `yaml:"volumes_root,omitempty"`
}

// DriverOrDefault returns the configured driver name, defaulting to SandboxDriverLocal.
func (c *SandboxConfig) DriverOrDefault() string {
	if c.Driver == "" {
		return SandboxDriverLocal
	}
	return c.Driver
}

// WorkspaceRoot returns the effective host-side workspace root: VolumesRoot if
// set, then $HOME, then ".". Remote drivers (docker/k8s) use this only when
// constructing local tool registries at startup; the per-session path inside
// the container is always ContainerWorkspaceRoot.
func (c *SandboxConfig) WorkspaceRoot() string {
	if c.VolumesRoot != "" {
		return c.VolumesRoot
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	return "."
}

// defaultConfigPaths lists candidate config file locations tried in order when
// no explicit path is provided. The first file that exists wins.
var defaultConfigPaths = []string{
	"forge.local.yaml",
	"configs/forge.yaml",
}

// Load reads configuration from path (may be empty) then fills any empty
// fields from environment variables.
// When path is empty the files in defaultConfigPaths are tried in order and
// the first one found is used.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	resolved := path
	if resolved == "" {
		for _, p := range defaultConfigPaths {
			if _, err := os.Stat(p); err == nil {
				resolved = p
				break
			}
		}
	}
	configDir := "."
	if resolved != "" {
		data, err := os.ReadFile(resolved)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", resolved, err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", resolved, err)
		}
		configDir = filepath.Dir(resolved)
	}
	if err := cfg.applyIncludes(configDir); err != nil {
		return nil, err
	}
	cfg.Tools.applyEnv()
	cfg.Log.applyDefaults()
	cfg.Auth.applyEnv()
	cfg.applyStorage()
	return cfg, nil
}

// applyIncludes loads MCP and PermissionFile when set, resolving paths
// relative to dir (the directory of the main config file).  Values from the
// included files overwrite the corresponding inline sections.
func (c *Config) applyIncludes(dir string) error {
	if c.MCP != "" {
		p := filepath.Join(dir, c.MCP)
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read mcp %s: %w", p, err)
		}
		if err := yaml.Unmarshal(data, &c.Tools); err != nil {
			return fmt.Errorf("parse mcp %s: %w", p, err)
		}
	}
	if c.PermissionFile != "" {
		p := filepath.Join(dir, c.PermissionFile)
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read permission %s: %w", p, err)
		}
		if err := yaml.Unmarshal(data, &c.Permission); err != nil {
			return fmt.Errorf("parse permission %s: %w", p, err)
		}
	}
	return nil
}

// applyStorage fills env fallbacks for named storage instances and resolves
// "type.name" driver references (e.g. "postgres.default", "redis.cache") into
// the actual driver name + connection options for each subsystem.
func (c *Config) applyStorage() {
	// ── env fallbacks for the "default" postgres instance ────────────────────
	if dsn := os.Getenv("FORGE_STORAGE_DSN"); dsn != "" {
		if c.Storage.Postgres == nil {
			c.Storage.Postgres = map[string]PostgresInstance{"default": {DSN: dsn}}
		} else if inst, ok := c.Storage.Postgres["default"]; ok && inst.DSN == "" {
			inst.DSN = dsn
			c.Storage.Postgres["default"] = inst
		}
	}

	// ── env fallbacks for the "default" redis instance ────────────────────────
	if c.Storage.Redis != nil {
		if inst, ok := c.Storage.Redis["default"]; ok {
			changed := false
			if inst.Addr == "" {
				if v := os.Getenv("FORGE_REDIS_ADDR"); v != "" {
					inst.Addr = v
					changed = true
				}
			}
			if inst.Password == "" {
				if v := os.Getenv("FORGE_REDIS_PASSWORD"); v != "" {
					inst.Password = v
					changed = true
				}
			}
			if changed {
				c.Storage.Redis["default"] = inst
			}
		}
	}

	// ── resolve "type.name" driver refs for every subsystem ──────────────────
	c.resolveDriver(&c.Session.Driver, &c.Session.Options)
	c.resolveDriver(&c.Memory.Driver, &c.Memory.Options)
	c.resolveDriver(&c.Tasks.Driver, &c.Tasks.Options)
	c.resolveDriver(&c.Store.Driver, &c.Store.Options)
}

// resolveDriver translates a "postgres.name" or "redis.name" driver string into
// the bare driver name ("postgres" / "redis") and injects the instance's
// connection parameters into opts. Bare drivers ("sqlite", "memory") are
// left unchanged.
func (c *Config) resolveDriver(driver *string, opts *map[string]string) {
	dot := strings.IndexByte(*driver, '.')
	if dot < 0 {
		return
	}
	typ, name := (*driver)[:dot], (*driver)[dot+1:]
	if *opts == nil {
		*opts = make(map[string]string)
	}
	switch typ {
	case "postgres":
		if c.Storage.Postgres != nil {
			if inst, ok := c.Storage.Postgres[name]; ok {
				if _, exists := (*opts)["dsn"]; !exists && inst.DSN != "" {
					(*opts)["dsn"] = inst.DSN
				}
				*driver = "postgres"
			}
		}
	case "redis":
		if c.Storage.Redis != nil {
			if inst, ok := c.Storage.Redis[name]; ok {
				if _, exists := (*opts)["addr"]; !exists && inst.Addr != "" {
					(*opts)["addr"] = inst.Addr
				}
				if _, exists := (*opts)["password"]; !exists && inst.Password != "" {
					(*opts)["password"] = inst.Password
				}
				if _, exists := (*opts)["db"]; !exists {
					(*opts)["db"] = fmt.Sprintf("%d", inst.DB)
				}
				*driver = "redis"
			}
		}
	}
}

// applyEnv fills empty ToolsConfig fields from environment variables.
func (t *ToolsConfig) applyEnv() {
	if t.WebSearch.Provider == "" {
		t.WebSearch.Provider = envOr("WEB_SEARCH_PROVIDER", "brave")
	}
	if t.WebSearch.APIKey == "" {
		switch t.WebSearch.Provider {
		case "serper":
			t.WebSearch.APIKey = os.Getenv("SERPER_API_KEY")
		default:
			t.WebSearch.APIKey = os.Getenv("BRAVE_API_KEY")
		}
	}
}

func (l *LogConfig) applyDefaults() {
	if l.Level == "" {
		l.Level = envOr("LOG_LEVEL", "info")
	}
	if l.Format == "" {
		l.Format = envOr("LOG_FORMAT", "text")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
