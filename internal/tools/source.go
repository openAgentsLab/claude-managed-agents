package tools

import (
	"context"
	"sync"
)

// ExecutionEnv declares what execution environment a tool source requires.
type ExecutionEnv int

const (
	// EnvInProcess tools run directly in the agent process — safe, no isolation
	// needed.  Examples: file operations, MCP tools, search tools.
	EnvInProcess ExecutionEnv = iota

	// EnvSandboxed tools require execution isolation and are routed through the
	// configured Sandbox (e.g. DockerSandbox).
	// Examples: bash, any future arbitrary-code execution tools.
	EnvSandboxed
)

// SourceFactory creates a ToolRegistry.
// It also returns a cleanup function (may be nil) that the caller must invoke
// on shutdown to release resources (e.g. close MCP server connections).
// Returning a nil ToolRegistry means this source contributes nothing
// (e.g. no MCP servers configured).
type SourceFactory func(ctx context.Context) (ToolRegistry, func())

type sourceEntry struct {
	factory SourceFactory
	env     ExecutionEnv
}

var (
	sourceMu      sync.RWMutex
	sourceEntries []sourceEntry

	readOnlyMu  sync.RWMutex
	readOnlySet = map[string]bool{}
)

// RegisterSource adds an in-process SourceFactory to the global registry.
// Tools from this source run directly in the agent process — no sandbox routing.
// Called from init() in each tool-source package; activated in main.go via
// blank imports (import _ "forge/internal/tools/local").
func RegisterSource(f SourceFactory) {
	addEntry(f, EnvInProcess)
}

// RegisterSandboxedSource adds a sandboxed SourceFactory to the global registry.
// Tools from this source are routed through the configured Sandbox for
// execution isolation (e.g. bash commands run inside a Docker container).
// In local dev mode (sandbox.driver = "local"), they still run in-process —
// the sandbox driver selection determines whether isolation is applied.
func RegisterSandboxedSource(f SourceFactory) {
	addEntry(f, EnvSandboxed)
}

func addEntry(f SourceFactory, env ExecutionEnv) {
	sourceMu.Lock()
	sourceEntries = append(sourceEntries, sourceEntry{factory: f, env: env})
	sourceMu.Unlock()
}

// RegisterReadOnly marks the named tools as read-only. Called from init() in
// tool-source packages alongside RegisterSource / RegisterSandboxedSource.
// Read-only tools are automatically allowed in plan mode by the permission engine.
func RegisterReadOnly(names ...string) {
	readOnlyMu.Lock()
	defer readOnlyMu.Unlock()
	for _, n := range names {
		readOnlySet[n] = true
	}
}

// ReadOnlyMap returns a snapshot of the registered read-only tool names.
// Pass this to permission.NewInterceptor instead of a hardcoded map so the
// permission engine stays in sync with the tool definitions.
func ReadOnlyMap() map[string]bool {
	readOnlyMu.RLock()
	defer readOnlyMu.RUnlock()
	out := make(map[string]bool, len(readOnlySet))
	for k, v := range readOnlySet {
		out[k] = v
	}
	return out
}

// Build assembles tool registries from all registered sources, split by
// execution environment:
//   - direct:    tools that run in the agent process (file ops, MCP, etc.)
//   - sandboxed: tools that need execution isolation (bash, code execution)
//
// Adding a new tool source only requires a blank import and a call to
// RegisterSource or RegisterSandboxedSource.
func Build(ctx context.Context) (direct, sandboxed ToolRegistry, cleanup func()) {
	sourceMu.RLock()
	entries := append([]sourceEntry(nil), sourceEntries...)
	sourceMu.RUnlock()

	var directRegs, sandboxedRegs []ToolRegistry
	var cleanups []func()

	for _, e := range entries {
		reg, c := e.factory(ctx)
		if reg != nil {
			if e.env == EnvSandboxed {
				sandboxedRegs = append(sandboxedRegs, reg)
			} else {
				directRegs = append(directRegs, reg)
			}
		}
		if c != nil {
			cleanups = append(cleanups, c)
		}
	}

	combined := func() {
		for _, c := range cleanups {
			c()
		}
	}
	return newCompositeRegistry(directRegs), newCompositeRegistry(sandboxedRegs), combined
}
