// Package hands implements the Sandbox layer from the managed-agents
// architecture: provision(tools) + execute(name, input) → string.
//
// Sandbox is pure execution — no tool declarations (those live in
// tools.ToolRegistry). LocalSandbox implements both interfaces; Brain receives
// them as two independent injections and is not aware they come from the same
// object.
package hands

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/eino/components/tool"
)

// Sandbox is the pure-execution abstraction. It does not expose tool
// declarations (those are in tools.ToolRegistry) and does not own Resources
// (those are assembled by Orchestration).
//
// Note: tool.Option values passed to sandboxTool.InvokableRun are forwarded for
// direct (in-process) tools but cannot be forwarded through Execute — the
// Sandbox interface has no opts parameter.
type Sandbox interface {
	// Provision initialises the sandbox for a session. Must be called once
	// before Execute. In-process sandboxes (local) build their tool dispatch
	// table here; remote sandboxes treat this as a no-op.
	Provision(ctx context.Context, execTools []tool.InvokableTool) error

	// Execute runs the named tool call and returns its output.
	Execute(ctx context.Context, name string, input json.RawMessage) (string, error)

	// Close releases resources held by the sandbox.
	Close() error
}

// HealthEndpointer is an optional interface for sandboxes that expose an HTTP
// health endpoint. Pool managers use this instead of type-asserting to a
// concrete type, keeping Healthy() decoupled from the implementation.
type HealthEndpointer interface {
	HealthEndpoint() string
}
