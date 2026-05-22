// Package mcp provides an MCP-backed tool source.  Registration is handled by
// tools/builtin.go — no blank import needed in main.go.
package mcp

import (
	"context"
	"log/slog"

	"github.com/cloudwego/eino/components/tool"

	mcpadapter "forge/internal/mcp/adapter"
	mcpclient "forge/internal/mcp/client"
	mcppolicy "forge/internal/mcp/policy"
)

// configKey is the context key used to inject platform-level MCP server configs.
type configKey struct{}

// WithConfig stores platform MCP server configs in ctx so that the
// RegisterSource callback in builtin.go can read them without touching
// the filesystem.  Called by hands.BuildSandboxLayer before tools.Build.
func WithConfig(ctx context.Context, servers map[string]mcpclient.MCPServerConfig) context.Context {
	return context.WithValue(ctx, configKey{}, servers)
}

func configFromContext(ctx context.Context) map[string]mcpclient.MCPServerConfig {
	m, _ := ctx.Value(configKey{}).(map[string]mcpclient.MCPServerConfig)
	return m
}

// Registry implements tools.ToolRegistry for MCP-connected tools.
type Registry struct {
	mgr *mcpclient.Manager
}

// NewRegistry connects to all platform-level MCP servers whose configs were
// injected into ctx by hands.BuildSandboxLayer via WithConfig.
// Returns (nil, nil, nil) when no servers are configured.
func NewRegistry(ctx context.Context) (*Registry, func(), error) {
	servers := configFromContext(ctx)

	var active []struct {
		name string
		cfg  mcpclient.MCPServerConfig
	}
	for name, cfg := range servers {
		if !cfg.Disabled {
			active = append(active, struct {
				name string
				cfg  mcpclient.MCPServerConfig
			}{name, cfg})
		}
	}
	if len(active) == 0 {
		return nil, nil, nil
	}

	filtered := mcppolicy.Filter(servers, mcppolicy.Settings{})
	if len(filtered) == 0 {
		return nil, nil, nil
	}

	mgr := mcpclient.NewManager()
	for name, cfg := range filtered {
		mgr.Add(name, cfg)
	}
	mgr.ConnectAll(ctx)

	reg := &Registry{mgr: mgr}
	slog.InfoContext(ctx, "mcp: platform tool source ready", "tools", len(reg.Tools()))
	return reg, mgr.Close, nil
}

// NewRegistryFromManager wraps an already-constructed and connected Manager.
// Used by the serve-mode session brain builder to attach user-configured MCP
// servers without re-loading settings from the filesystem.
func NewRegistryFromManager(mgr *mcpclient.Manager) *Registry {
	return &Registry{mgr: mgr}
}

// Tools implements tools.ToolRegistry.
func (r *Registry) Tools() []tool.BaseTool {
	var out []tool.BaseTool
	out = append(out, mcpadapter.NewTools(r.mgr)...)
	out = append(out, mcpadapter.NewResourceTools(r.mgr)...)
	return out
}
