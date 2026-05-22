package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	mcpclient "forge/internal/mcp/client"
)

// ── ListMcpResourcesTool ─────────────────────────────────────────────────────

type listResourcesInput struct {
	Server string `json:"server,omitempty"`
}

type listResourcesTool struct {
	mgr  *mcpclient.Manager
	info *schema.ToolInfo
}

// NewListMcpResourcesTool returns an Eino tool that lists available resources
// across all connected MCP servers, optionally filtered by server name.
func NewListMcpResourcesTool(mgr *mcpclient.Manager) tool.BaseTool {
	return &listResourcesTool{
		mgr: mgr,
		info: &schema.ToolInfo{
			Name: "mcp_list_resources",
			Desc: "Lists available resources (files, data, etc.) from connected MCP servers. Each resource has a URI that can be passed to mcp_read_resource to fetch its content.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"server": {
					Type:     schema.String,
					Desc:     "Optional: filter results to this MCP server name. Omit to list resources from all connected servers.",
					Required: false,
				},
			}),
		},
	}
}

func (t *listResourcesTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *listResourcesTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var input listResourcesInput
	if argsJSON != "" && argsJSON != "{}" {
		if err := json.Unmarshal([]byte(argsJSON), &input); err != nil {
			return "", fmt.Errorf("mcp_list_resources: invalid args: %w", err)
		}
	}

	slog.DebugContext(ctx, "mcp list resources", "server", input.Server)
	resources, err := t.mgr.ListResources(ctx, input.Server)
	if err != nil {
		slog.WarnContext(ctx, "mcp list resources failed", "server", input.Server, "error", err)
		return "", fmt.Errorf("mcp_list_resources: %w", err)
	}
	slog.DebugContext(ctx, "mcp list resources done", "count", len(resources))
	if len(resources) == 0 {
		if input.Server != "" {
			return fmt.Sprintf("No resources found on server %q.", input.Server), nil
		}
		return "No resources found on any connected MCP server.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d resource(s):\n\n", len(resources)))
	for _, r := range resources {
		sb.WriteString(fmt.Sprintf("server: %s\n", r.ServerName))
		sb.WriteString(fmt.Sprintf("uri:    %s\n", r.URI))
		sb.WriteString(fmt.Sprintf("name:   %s\n", r.Name))
		if r.MIMEType != "" {
			sb.WriteString(fmt.Sprintf("mime:   %s\n", r.MIMEType))
		}
		if r.Description != "" {
			sb.WriteString(fmt.Sprintf("desc:   %s\n", r.Description))
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

// ── ReadMcpResourceTool ──────────────────────────────────────────────────────

type readResourceInput struct {
	Server string `json:"server"`
	URI    string `json:"uri"`
}

type readResourceTool struct {
	mgr  *mcpclient.Manager
	info *schema.ToolInfo
}

// NewReadMcpResourceTool returns an Eino tool that reads a specific MCP resource
// by server name and URI.
func NewReadMcpResourceTool(mgr *mcpclient.Manager) tool.BaseTool {
	return &readResourceTool{
		mgr: mgr,
		info: &schema.ToolInfo{
			Name: "mcp_read_resource",
			Desc: "Reads the content of a specific resource from an MCP server. Use mcp_list_resources first to discover available resource URIs.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"server": {
					Type:     schema.String,
					Desc:     "The MCP server name that owns this resource.",
					Required: true,
				},
				"uri": {
					Type:     schema.String,
					Desc:     "The URI of the resource to read, as returned by mcp_list_resources.",
					Required: true,
				},
			}),
		},
	}
}

func (t *readResourceTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return t.info, nil
}

func (t *readResourceTool) InvokableRun(ctx context.Context, argsJSON string, _ ...tool.Option) (string, error) {
	var input readResourceInput
	if err := json.Unmarshal([]byte(argsJSON), &input); err != nil {
		return "", fmt.Errorf("mcp_read_resource: invalid args: %w", err)
	}
	if input.Server == "" {
		return "", fmt.Errorf("mcp_read_resource: 'server' is required")
	}
	if input.URI == "" {
		return "", fmt.Errorf("mcp_read_resource: 'uri' is required")
	}

	slog.DebugContext(ctx, "mcp read resource", "server", input.Server, "uri", input.URI)
	text, mimeType, isBlob, err := t.mgr.ReadResource(ctx, input.Server, input.URI)
	if err != nil {
		slog.WarnContext(ctx, "mcp read resource failed", "server", input.Server, "uri", input.URI, "error", err)
		return "", fmt.Errorf("mcp_read_resource: %w", err)
	}
	slog.DebugContext(ctx, "mcp read resource done",
		"server", input.Server,
		"uri", input.URI,
		"mime", mimeType,
		"blob", isBlob,
		"len", len(text),
	)

	if isBlob {
		hint := ""
		if mimeType != "" {
			hint = fmt.Sprintf(" (mime: %s)", mimeType)
		}
		return fmt.Sprintf("[Binary resource%s — base64 content below]\n%s", hint, text), nil
	}

	if mimeType != "" && mimeType != "text/plain" {
		return fmt.Sprintf("[%s]\n%s", mimeType, text), nil
	}
	return text, nil
}

// NewResourceTools returns both resource tools as a slice ready to inject into
// the Eino agent alongside the regular MCP tool wrappers.
func NewResourceTools(mgr *mcpclient.Manager) []tool.BaseTool {
	return []tool.BaseTool{
		NewListMcpResourcesTool(mgr),
		NewReadMcpResourceTool(mgr),
	}
}
