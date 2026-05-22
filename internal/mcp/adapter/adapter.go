// Package adapter maps MCP tools to the internal unified tool interface.
//
// Each MCP ToolEntry is wrapped in an mcpTool that implements
// tool.InvokableTool so it can be passed directly to the Eino ReAct agent
// alongside built-in tools.
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	jsonschema "github.com/eino-contrib/jsonschema"

	mcpclient "forge/internal/mcp/client"
)

// mcpTool wraps a single MCP ToolEntry as an Eino InvokableTool.
type mcpTool struct {
	entry      mcpclient.ToolEntry
	serverName string
	mgr        *mcpclient.Manager
	info       *schema.ToolInfo
}

// NewTools converts the Manager's current connected tool set into Eino
// tool.BaseTool values.  Call this after ConnectAll (or after a toolUpdateCh
// signal) to refresh the tool slice.
func NewTools(mgr *mcpclient.Manager) []tool.BaseTool {
	entries := mgr.AllTools()
	out := make([]tool.BaseTool, 0, len(entries))
	for _, e := range entries {
		serverName, _, ok := mcpclient.SplitMCPToolName(e.NormalizedName)
		if !ok {
			slog.Warn("mcp adapter: skipping tool with unparseable name", "name", e.NormalizedName)
			continue
		}
		t := buildTool(e, serverName, mgr)
		out = append(out, t)
	}
	slog.Debug("mcp adapter: tools registered", "count", len(out))
	return out
}

func buildTool(e mcpclient.ToolEntry, serverName string, mgr *mcpclient.Manager) *mcpTool {
	// Marshal the MCP input schema to JSON, decode into *jsonschema.Schema,
	// then pass to Eino for verbatim passthrough to the model.
	var paramsOneOf *schema.ParamsOneOf
	if raw, err := json.Marshal(e.InputSchema); err == nil && string(raw) != "null" {
		var js jsonschema.Schema
		if jsonErr := json.Unmarshal(raw, &js); jsonErr == nil {
			paramsOneOf = schema.NewParamsOneOfByJSONSchema(&js)
		} else {
			slog.Warn("mcp adapter: failed to parse input schema for tool; model will receive no parameter hints",
				"tool", e.NormalizedName,
				"error", jsonErr,
			)
		}
	}

	info := &schema.ToolInfo{
		Name:        e.NormalizedName,
		Desc:        e.Description,
		ParamsOneOf: paramsOneOf,
	}

	return &mcpTool{
		entry:      e,
		serverName: serverName,
		mgr:        mgr,
		info:       info,
	}
}

// Info implements tool.BaseTool.
func (t *mcpTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

// InvokableRun implements tool.InvokableTool.
func (t *mcpTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	slog.DebugContext(ctx, "mcp tool call",
		"tool", t.entry.NormalizedName,
		"server", t.serverName,
	)
	result, err := t.mgr.CallTool(ctx, t.serverName, t.entry.OriginalName, argsJSON)
	if err != nil {
		slog.WarnContext(ctx, "mcp tool call failed",
			"tool", t.entry.NormalizedName,
			"server", t.serverName,
			"error", err,
		)
		return "", fmt.Errorf("mcp tool %s: %w", t.entry.NormalizedName, err)
	}
	slog.DebugContext(ctx, "mcp tool call succeeded",
		"tool", t.entry.NormalizedName,
		"response_len", len(result),
	)
	return result, nil
}
