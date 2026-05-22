package tools

import "github.com/cloudwego/eino/components/tool"

// ToolRegistry is the source of tool instances for Brain.
// Every implementation returns ready-to-use Eino tool.BaseTool values that
// carry both declaration (Info) and execution logic (InvokableRun).
// Implementations: tools/local.LocalRegistry, tools/mcp.mcpRegistry.
type ToolRegistry interface {
	Tools() []tool.BaseTool
}

// Static wraps a fixed slice of tools in a ToolRegistry.
// Used by middleware layers that transform a registry's tools and need to
// return the result as a new ToolRegistry without adding a new named type.
func Static(ts []tool.BaseTool) ToolRegistry {
	return &staticRegistry{tools: ts}
}

type staticRegistry struct{ tools []tool.BaseTool }

func (s *staticRegistry) Tools() []tool.BaseTool { return s.tools }
