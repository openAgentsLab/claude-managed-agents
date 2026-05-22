package client

// MCPServerType identifies the transport used to reach an MCP server.
type MCPServerType string

const (
	MCPStdio MCPServerType = "stdio"
	MCPSSE   MCPServerType = "sse"
	MCPHTTP  MCPServerType = "http"
	MCPWS    MCPServerType = "ws"
)

// MCPStdioConfig is the configuration for a stdio-transport MCP server.
// The server is started as a child process; communication happens over
// its stdin/stdout using the JSON-RPC MCP protocol.
type MCPStdioConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// MCPRemoteConfig is the configuration for SSE / HTTP / WebSocket servers.
type MCPRemoteConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// MCPServerConfig is the union type stored in settings.json under
// "mcpServers".  Exactly one of StdioConfig or RemoteConfig is non-nil
// depending on Type.
type MCPServerConfig struct {
	Type     MCPServerType `json:"type"`
	Disabled bool          `json:"disabled,omitempty"`

	// Stdio
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	// Remote
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ServerState is the lifecycle state of a managed MCP server connection.
type ServerState int

const (
	StateDisconnected ServerState = iota
	StatePending
	StateConnected
	StateFailed
)
