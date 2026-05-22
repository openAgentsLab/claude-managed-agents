package tools

import "github.com/cloudwego/eino/components/tool"

// compositeRegistry merges Tools() from all child registries.
type compositeRegistry struct {
	registries []ToolRegistry
}

func newCompositeRegistry(registries []ToolRegistry) ToolRegistry {
	return &compositeRegistry{registries: registries}
}

// Tools implements ToolRegistry — collects Eino tools from all children.
func (c *compositeRegistry) Tools() []tool.BaseTool {
	var all []tool.BaseTool
	for _, r := range c.registries {
		all = append(all, r.Tools()...)
	}
	return all
}

// Merge combines multiple ToolRegistries into a single one whose Tools()
// returns the concatenation of all inputs.  Used by buildRegistry to combine
// the direct and sandboxed registries after applying the execution policy.
func Merge(regs ...ToolRegistry) ToolRegistry {
	return newCompositeRegistry(regs)
}
