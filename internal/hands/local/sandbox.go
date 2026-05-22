package local

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
)

// LocalSandbox implements hands.Sandbox — pure tool execution scoped to a
// workspace. Tool declarations live in tools/local.LocalRegistry; the two are
// independent and injected separately into Brain.
type LocalSandbox struct {
	toolMap map[string]tool.InvokableTool
}

// NewLocalSandbox creates an uninitialised LocalSandbox.
func NewLocalSandbox() *LocalSandbox {
	return &LocalSandbox{}
}

// Provision builds the execution index from the workspace tools.
func (s *LocalSandbox) Provision(ctx context.Context, execTools []tool.InvokableTool) error {
	s.toolMap = indexByName(ctx, execTools)
	return nil
}

// Execute implements hands.Sandbox — dispatches by name.
func (s *LocalSandbox) Execute(ctx context.Context, name string, input json.RawMessage) (string, error) {
	inv, ok := s.toolMap[name]
	if !ok {
		return "", fmt.Errorf("sandbox: tool %q not found", name)
	}
	return inv.InvokableRun(ctx, string(input))
}

// Close releases sandbox resources.
func (s *LocalSandbox) Close() error { return nil }

func indexByName(ctx context.Context, tools []tool.InvokableTool) map[string]tool.InvokableTool {
	m := make(map[string]tool.InvokableTool, len(tools))
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil || info == nil {
			continue
		}
		m[info.Name] = t
	}
	return m
}
